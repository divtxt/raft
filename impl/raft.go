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
//  - ChangeListener
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
	"errors"
	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/config"
	"github.com/divtxt/raft/consensus"
	"github.com/divtxt/raft/util"
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
	stopped bool

	// -- Ticker
	tickerDuration time.Duration
	ticker         *util.Ticker
}

// Allocate and initialize a ConsensusModule with the given components and
// settings.
//
// All parameters are required.
// timeSettings is checked using ValidateTimeSettings().
//
func NewConsensusModule(
	raftPersistentState RaftPersistentState,
	log Log,
	rpcService RpcService,
	clusterInfo *config.ClusterInfo,
	maxEntriesPerAppendEntry uint64,
	timeSettings config.TimeSettings,
) (*ConsensusModule, error) {
	now := time.Now()

	cm := &ConsensusModule{
		&sync.Mutex{},

		nil, // temp value, to be replaced before goroutine start

		// -- External components
		rpcService,

		// -- State
		true,

		// -- Ticker
		timeSettings.TickerDuration,
		nil,
	}

	pcm, err := consensus.NewPassiveConsensusModule(
		raftPersistentState,
		log,
		nil,
		cm,
		clusterInfo,
		maxEntriesPerAppendEntry,
		timeSettings.ElectionTimeoutLow,
		now,
	)
	if err != nil {
		return nil, err
	}

	// we can only set the value here because it's a cyclic reference
	cm.passiveConsensusModule = pcm

	return cm, nil
}

// Start the ConsensusModule running with the given ChangeListener.
//
// This starts a goroutine that drives ticks.
//
// Should only be called once.
func (cm *ConsensusModule) Start(changeListener ChangeListener) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if changeListener == nil {
		return errors.New("'changeListener' cannot be nil")
	}

	if cm.ticker != nil {
		return ErrAlreadyStartedOnce
	}

	cm.passiveConsensusModule.SetChangeListener(changeListener)

	cm.stopped = false

	// Start the ticker goroutine
	cm.ticker = util.NewTicker(cm.safeTick, cm.tickerDuration)

	return nil
}

// Check if the ConsensusModule is stopped.
func (cm *ConsensusModule) IsStopped() bool {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	return cm.stopped
}

// Stop the ConsensusModule.
//
// This will mark the ConsensusModule as stopped and stop the goroutine that does the processing.
// This call is effectively synchronous as far as the ConsensusModule operation is concerned,
// even though the goroutine may not stop immediately.
// This is safe to call multiple times, even if the ConsensusModule has already stopped.
func (cm *ConsensusModule) Stop() {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	cm.shutdownAndPanic(nil)
}

// Get the current server state.
//
// This value is irrelevant if the ConsensusModule is stopped.
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
		cm.shutdownAndPanic(err)
		return nil // unreachable code
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
		cm.shutdownAndPanic(err)
		return nil // unreachable code
	}

	return rpcReply
}

// Append the given command as an entry in the log.
//
// This can only be done if the ConsensusModule is in LEADER state.
//
// The command will be sent to Log.AppendEntry().
//
// The command must already have been checked to ensure that it will successfully apply to the
// state machine in it's position in the Log.
//
// Returns ErrStopped if ConsensusModule is stopped.
// Returns ErrNotLeader if not currently the leader.
//
// Any errors from Log.AppendCommand() call will stop the ConsensusModule.
//
// Here, we intentionally punt on some of the leader details, specifically
// most of:
//
// #RFS-L2: If command received from client: append entry to local log,
// respond after entry applied to state machine (#5.3)
//
// We choose not to deal with the client directly. You must implement the interaction with
// clients and, if required, with waiting for the entry to be applied to the state machine.
// (see delegation of lastApplied to the state machine via the ChangeListener interface)
//
// See the notes on NewConsensusModule() for more details about this method's behavior.
func (cm *ConsensusModule) AppendCommand(command Command) (LogIndex, error) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if cm.stopped {
		return 0, ErrStopped
	}

	iole, err := cm.passiveConsensusModule.AppendCommand(command)
	if err != nil && err != ErrNotLeader {
		cm.shutdownAndPanic(err)
	}

	return iole, err
}

// -- protected methods

// Implement RpcSendOnly.SendOnlyRpcAppendEntriesAsync to bridge to
// RpcService.RpcAppendEntries() with a closure callback.
func (cm *ConsensusModule) SendOnlyRpcAppendEntriesAsync(
	toServer ServerId,
	rpc *RpcAppendEntries,
) {
	rpcAndCallback := func() {
		// Make the RPC call
		rpcReply := cm.rpcService.RpcAppendEntries(toServer, rpc)

		// If successful, send it back to the ConsensusModule
		if rpcReply != nil {
			cm.safeProcessRpcReply_RpcAppendEntriesReply(toServer, rpc, rpcReply)
		}
	}
	go rpcAndCallback()
}

func (cm *ConsensusModule) safeProcessRpcReply_RpcAppendEntriesReply(
	fromPeer ServerId,
	rpc *RpcAppendEntries,
	rpcReply *RpcAppendEntriesReply,
) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if !cm.stopped {
		err := cm.passiveConsensusModule.RpcReply_RpcAppendEntriesReply(fromPeer, rpc, rpcReply)
		if err != nil {
			cm.shutdownAndPanic(err)
		}
	}
}

// Implement RpcSendOnly.SendOnlyRpcRequestVoteAsync to bridge to
// RpcService.RpcRequestVote() with a closure callback.
func (cm *ConsensusModule) SendOnlyRpcRequestVoteAsync(
	toServer ServerId,
	rpc *RpcRequestVote,
) {
	rpcAndCallback := func() {
		// Make the RPC call
		rpcReply := cm.rpcService.RpcRequestVote(toServer, rpc)

		// If successful, send it back to the ConsensusModule
		if rpcReply != nil {
			cm.safeProcessRpcReply_RpcRequestVoteReply(toServer, rpc, rpcReply)
		}
	}
	go rpcAndCallback()
}

func (cm *ConsensusModule) safeProcessRpcReply_RpcRequestVoteReply(
	fromPeer ServerId,
	rpc *RpcRequestVote,
	rpcReply *RpcRequestVoteReply,
) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if !cm.stopped {
		err := cm.passiveConsensusModule.RpcReply_RpcRequestVoteReply(fromPeer, rpc, rpcReply)
		if err != nil {
			cm.shutdownAndPanic(err)
		}
	}
}

func (cm *ConsensusModule) safeTick() {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if !cm.stopped {
		// Get a fresh now since we could have been waiting
		now := time.Now()
		err := cm.passiveConsensusModule.Tick(now)
		if err != nil {
			cm.shutdownAndPanic(err)
		}
	}
}

// Shutdown the ConsensusModule.
// Panic if the given error is not nil.
func (cm *ConsensusModule) shutdownAndPanic(err error) {
	if !cm.stopped {
		// Tell the ticker to stop
		cm.ticker.StopAsync()
		// Update state
		cm.stopped = true
		// Panic for error
		if err != nil {
			panic(err)
		}
	}
}
