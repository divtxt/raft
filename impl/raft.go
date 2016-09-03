// Package raft is an implementation of the Raft consensus protocol.
//
// Call NewConsensusModule with appropriate parameters to start an instance.
// Incoming RPC calls can then be sent to it using the ProcessRpc...Async
// methods.
//
// You will have to provide implementations of the following interfaces:
//
//  - RaftPersistentState
//  - Log
//  - StateMachine
//  - RpcService
//
// Notes for implementers of these interfaces:
//
// - Concurrency: a ConsensusModule will only ever call the methods of these
// interfaces from it's single goroutine.
//
// - Errors: all errors should be checked and returned. This includes both
// invalid parameters sent by the consensus module and internal errors in the
// implementation. Note that any error will shutdown the ConsensusModule.
//
package impl

import (
	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/config"
	"github.com/divtxt/raft/consensus"
	"sync"
	"time"
)

// A ConsensusModule is an active Raft consensus module implementation.
type ConsensusModule struct {
	mutex *sync.Mutex

	//
	passiveConsensusModule *consensus.PassiveConsensusModule

	// -- External components - these fields meant to be immutable
	rpcService RpcService

	// -- State
	stopped   bool
	stopError error

	// -- Ticker
	ticker       *time.Ticker
	stopSignal   chan struct{}
	runTicksDone *sync.WaitGroup
}

// Allocate and initialize a ConsensusModule with the given components and
// settings.
//
// All parameters are required.
// timeSettings is checked using ValidateTimeSettings().
//
// A goroutine that drives ticks is created.
//
func NewConsensusModule(
	raftPersistentState RaftPersistentState,
	log Log,
	stateMachine StateMachine,
	rpcService RpcService,
	clusterInfo *config.ClusterInfo,
	timeSettings config.TimeSettings,
) (*ConsensusModule, error) {
	ticker := time.NewTicker(timeSettings.TickerDuration)
	now := time.Now()

	cm := &ConsensusModule{
		&sync.Mutex{},

		nil, // temp value, to be replaced before goroutine start

		// -- External components
		rpcService,

		// -- State
		false,
		nil,

		// -- Ticker
		ticker,
		make(chan struct{}, 1),
		&sync.WaitGroup{},
	}

	pcm, err := consensus.NewPassiveConsensusModule(
		raftPersistentState,
		log,
		stateMachine,
		cm,
		clusterInfo,
		timeSettings.ElectionTimeoutLow,
		now,
	)
	if err != nil {
		return nil, err
	}

	// we can only set the value here because it's a cyclic reference
	cm.passiveConsensusModule = pcm

	// Start the ticker go routine
	cm.runTicksDone.Add(1)
	go cm.runTicks()

	return cm, nil
}

// Check if the ConsensusModule is stopped.
func (cm *ConsensusModule) IsStopped() bool {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	return cm.stopped
}

// Stop the ConsensusModule.
//
// This will effectively stop the goroutine that does the processing.
// This is safe to call multiple times, even if the ConsensusModule has already stopped.
func (cm *ConsensusModule) Stop() {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	cm.shutdown(nil)
}

// Get the error that stopped the ConsensusModule.
//
// The value will be nil if the ConsensusModule is not stopped, or if it stopped
// without an error.
func (cm *ConsensusModule) GetStopError() error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	return cm.stopError
}

// Get the current server state.
func (cm *ConsensusModule) GetServerState() ServerState {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	return cm.passiveConsensusModule.GetServerState()
}

// Process the given RpcAppendEntries message from the given peer.
//
// Returns nil if there was an error or if the ConsensusModule is shutdown.
//
// Note that an error would have shutdown the ConsensusModule.
func (cm *ConsensusModule) ProcessRpcAppendEntries(
	from ServerId,
	rpc *RpcAppendEntries,
) *RpcAppendEntriesReply {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if cm.stopped {
		return nil
	}

	now := time.Now()

	rpcReply, err := cm.passiveConsensusModule.Rpc_RpcAppendEntries(from, rpc, now)
	if err != nil {
		cm.shutdown(err)
		return nil
	}

	return rpcReply
}

// Process the given RpcRequestVote message from the given peer
// asynchronously.
//
// This method sends the RPC message to the ConsensusModule's goroutine.
// The RPC reply will be sent later on the returned channel.
//
// See the RpcService interface for outgoing RPC.
//
// See the notes on NewConsensusModule() for more details about this method's behavior.
func (cm *ConsensusModule) ProcessRpcRequestVote(
	from ServerId,
	rpc *RpcRequestVote,
) *RpcRequestVoteReply {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if cm.stopped {
		return nil
	}

	now := time.Now()

	rpcReply, err := cm.passiveConsensusModule.Rpc_RpcRequestVote(from, rpc, now)
	if err != nil {
		cm.shutdown(err)
		return nil
	}

	return rpcReply
}

// Append the given command as an entry in the log.
//
// This can only be done if the ConsensusModule is in LEADER state.
//
// The command will be sent to Log.AppendEntry().
//
// Returns ErrStopped if ConsensusModule is stopped.
// Returns ErrNotLeader if not currently the leader.
//
// Here, we intentionally punt on some of the leader details, specifically
// most of:
//
// #RFS-L2: If command received from client: append entry to local log,
// respond after entry applied to state machine (#5.3)
//
// We choose not to deal with the client directly. You must implement the interaction with
// clients and with waiting for the entry to be applied to the state machine.
// (see delegation of lastApplied to the StateMachine interface)
//
// See the notes on NewConsensusModule() for more details about this method's behavior.
func (cm *ConsensusModule) AppendCommand(command Command) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if cm.stopped {
		return nil
	}

	err := cm.passiveConsensusModule.AppendCommand(command)
	// FIXME: differentiate errors

	return err
}

// -- protected methods

// Implement RpcSendOnly.SendOnlyRpcAppendEntriesAsync to bridge to
// RpcService.RpcAppendEntries() with a closure callback.
func (cm *ConsensusModule) SendOnlyRpcAppendEntriesAsync(
	toServer ServerId,
	rpc *RpcAppendEntries,
) error {
	rpcAndCallback := func() {
		// Make the RPC call
		rpcReply := cm.rpcService.RpcAppendEntries(toServer, rpc)

		// If successful, send it back to the ConsensusModule
		if rpcReply != nil {
			cm.passiveConsensusModule.RpcReply_RpcAppendEntriesReply(toServer, rpc, rpcReply)
		}
		// go cm.safeRunWithErrCheck(f)
	}
	go rpcAndCallback()
	return nil
}

// Implement RpcSendOnly.SendOnlyRpcRequestVoteAsync to bridge to
// RpcService.RpcRequestVote() with a closure callback.
func (cm *ConsensusModule) SendOnlyRpcRequestVoteAsync(
	toServer ServerId,
	rpc *RpcRequestVote,
) error {
	rpcAndCallback := func() {
		// Make the RPC call
		rpcReply := cm.rpcService.RpcRequestVote(toServer, rpc)

		// If successful, send it back to the ConsensusModule
		if rpcReply != nil {
			cm.passiveConsensusModule.RpcReply_RpcRequestVoteReply(toServer, rpc, rpcReply)
		}
	}
	go rpcAndCallback()
	return nil
}

func (cm *ConsensusModule) runTicks() {
	defer cm.runTicksDone.Done()
	defer cm.ticker.Stop()

loop:
	for {
		select {
		case _, ok := <-cm.ticker.C:
			if !ok {
				break loop
			}
			cm.safeTick()
		case <-cm.stopSignal:
			break loop
		}
	}
}

func (cm *ConsensusModule) safeTick() {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// Get a fresh now since we could have been waiting
	now := time.Now()
	cm.passiveConsensusModule.Tick(now)
}

func (cm *ConsensusModule) shutdown(err error) {
	if !cm.stopped {
		// Stop the ticker
		select {
		case cm.stopSignal <- struct{}{}:
		default:
		}
		// Wait for ticker goroutine to actually stop
		cm.runTicksDone.Wait()
		// Update state
		cm.stopError = err
		cm.stopped = true
	}
}
