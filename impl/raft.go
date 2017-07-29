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
	"log"
	"sync"
	"time"

	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/committer"
	"github.com/divtxt/raft/config"
	"github.com/divtxt/raft/consensus"
	"github.com/divtxt/raft/util"
)

// A ConsensusModule is an active Raft consensus module implementation.
type ConsensusModule struct {
	mutex *sync.Mutex

	//
	logger *log.Logger

	//
	committer              *committer.Committer
	passiveConsensusModule *consensus.PassiveConsensusModule

	// -- External components - these fields meant to be immutable
	rpcService RpcService

	// -- State
	stopped bool

	// -- Ticker
	tickerDuration time.Duration
	ticker         *util.Ticker
}

// NewConsensusModule creates and starts a ConsensusModule with the given components and
// settings.
//
// All parameters are required.
// timeSettings is checked using ValidateTimeSettings().
//
// The goroutine that drives ticks (and therefore RPCs) is started.
//
func NewConsensusModule(
	raftPersistentState RaftPersistentState,
	raftLog Log,
	stateMachine StateMachine,
	rpcService RpcService,
	clusterInfo *config.ClusterInfo,
	maxEntriesPerAppendEntry uint64,
	timeSettings config.TimeSettings,
	logger *log.Logger,
) (*ConsensusModule, error) {
	logger.Println("[raft] Initializing ConsensusModule")

	committer := committer.NewCommitter(raftLog, stateMachine)

	cm := &ConsensusModule{
		&sync.Mutex{},

		logger,

		committer,
		nil, // passiveConsensusModule - temp value, to be replaced before goroutine start

		// -- External components
		rpcService,

		// -- State
		false, // stopped flag

		// -- Ticker
		timeSettings.TickerDuration,
		nil,
	}

	pcm, err := consensus.NewPassiveConsensusModule(
		raftPersistentState,
		raftLog,
		committer,
		cm,
		clusterInfo,
		maxEntriesPerAppendEntry,
		timeSettings.ElectionTimeoutLow,
		time.Now,
		logger,
	)
	if err != nil {
		return nil, err
	}

	// We can only set the value here because it's a cyclic reference
	cm.passiveConsensusModule = pcm

	// Start the ticker goroutine
	cm.ticker = util.NewTicker(cm.safeTick, cm.tickerDuration)

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
// This will mark the ConsensusModule as stopped and stop the goroutine that does the processing.
// This call is effectively synchronous as far as the ConsensusModule operation is concerned,
// even though the goroutine may not stop immediately.
// This is safe to call multiple times, even if the ConsensusModule has already stopped.
func (cm *ConsensusModule) Stop() {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	cm.logger.Println("[raft] Stopping ConsensusModule")
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

	rpcReply, err := cm.passiveConsensusModule.Rpc_RpcAppendEntries(from, rpc)
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

	rpcReply, err := cm.passiveConsensusModule.Rpc_RpcRequestVote(from, rpc)
	if err != nil {
		cm.shutdownAndPanic(err)
		return nil // unreachable code
	}

	return rpcReply
}

// AppendCommand appends the given serialized command to the log.
func (cm *ConsensusModule) AppendCommand(command Command) (<-chan CommandResult, error) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if cm.stopped {
		return nil, ErrStopped
	}

	crc, err := cm.passiveConsensusModule.AppendCommand(command)
	if err != nil && err != ErrNotLeader {
		cm.shutdownAndPanic(err)
	}

	return crc, err
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
		err := cm.passiveConsensusModule.Tick()
		if err != nil {
			cm.shutdownAndPanic(err)
		}
	}
}

// Shutdown the ConsensusModule.
// Panic if the given error is not nil.
// It is expected that we're under mutex when this method is called.
func (cm *ConsensusModule) shutdownAndPanic(err error) {
	if !cm.stopped {
		// Mark self as stopped.
		// Since we should be under mutex, no other calls will be serviced after this line.
		cm.stopped = true
		// Tell the ticker to stop.
		// This needs be async since this method could be running as part of a tick.
		cm.ticker.StopAsync()
		// Tell the committer to stop.
		// No other calls will be serviced, so there's no need to worry about a race condition
		// between this stop and a commitIndex change.
		// (Even if this method is running as part of a tick, we should be past the actual tick code)
		// where the actual tick code
		cm.committer.StopSync()
		// Panic for error
		if err != nil {
			panic(err)
		}
	}
}
