// Package raft is an implementation of the Raft consensus protocol.
//
// Call NewConsensusModule with appropriate parameters to start an instance.
// Incoming RPC calls can then be sent to it using the ProcessRpc...Async
// methods.
//
// You will have to provide implementations of the following interfaces:
//
//  - raft.PersistentState
//  - raft.Log
//  - raft.RpcService
//
// Notes for implementers of these interfaces:
//
// - Concurrency: a ConsensusModule will only ever call the methods of these
// interfaces from it's single goroutine.
//
// - Errors: all errors should be indicated using panic(). This includes both
// invalid parameters sent by the consensus module and internal errors in the
// implementation. Note that such a panic will shutdown the ConsensusModule.
//
package raft

import (
	"sync/atomic"
	"time"
)

// A ConsensusModule is an active Raft consensus module implementation.
type ConsensusModule struct {
	passiveConsensusModule *passiveConsensusModule

	// -- External components - these fields meant to be immutable
	rpcService RpcService

	// -- State - these fields may be accessed concurrently
	stopped int32

	// -- Channels
	runnableChannel chan func()
	ticker          *time.Ticker

	// -- Control
	stopSignal chan struct{}
	stopError  *atomic.Value
}

// Allocate and initialize a ConsensusModule with the given components and
// settings.
//
// A goroutine that handles consensus processing is created.
// All parameters are required.
// timeSettings is checked using ValidateTimeSettings().
func NewConsensusModule(
	persistentState PersistentState,
	log Log,
	rpcService RpcService,
	clusterInfo *ClusterInfo,
	timeSettings TimeSettings,
) *ConsensusModule {
	runnableChannel := make(chan func(), RPC_CHANNEL_BUFFER_SIZE)
	ticker := time.NewTicker(timeSettings.TickerDuration)
	now := time.Now()

	cm := &ConsensusModule{
		nil, // temp value, to be replaced before goroutine start

		// -- External components
		rpcService,

		// -- State
		0,

		// -- Channels
		runnableChannel,
		ticker,

		// -- Control
		make(chan struct{}),
		&atomic.Value{},
	}

	pcm := newPassiveConsensusModule(
		persistentState,
		log,
		cm,
		clusterInfo,
		timeSettings.ElectionTimeoutLow,
		now,
	)

	// we can only set the value here because it's a cyclic reference
	cm.passiveConsensusModule = pcm

	// Start the go routine
	go cm.processor()

	return cm
}

// Check if the ConsensusModule is stopped.
func (cm *ConsensusModule) IsStopped() bool {
	return atomic.LoadInt32(&cm.stopped) != 0
}

// Stop the ConsensusModule asynchronously.
//
// This will stop the goroutine that does the processing.
// This is safe to call even if the goroutine has already stopped, but it
// will panic if called more than once.
func (cm *ConsensusModule) StopAsync() {
	close(cm.stopSignal)
}

// Get the error that stopped the ConsensusModule goroutine.
//
// Gets the recover value for the panic that stopped the goroutine.
// The value will be nil if the goroutine is not stopped, stopped
// without an error, or  panicked with a nil value.
func (cm *ConsensusModule) GetStopError() interface{} {
	return cm.stopError.Load()
}

// Get the current server state.
func (cm *ConsensusModule) GetServerState() ServerState {
	return cm.passiveConsensusModule.getServerState()
}

// Process the given RpcAppendEntries message from the given peer
// asynchronously.
//
// This method sends the RPC message to the ConsensusModule's goroutine.
// The RPC reply will be sent later on the returned channel.
//
// See RpcSender (in interfaces.go) for outgoing RPC.
//
// TODO: behavior when channel full?
func (cm *ConsensusModule) ProcessRpcAppendEntriesAsync(
	from ServerId,
	rpc *RpcAppendEntries,
) <-chan *RpcAppendEntriesReply {
	replyChan := make(chan *RpcAppendEntriesReply, 1)
	cm.runnableChannel <- func() {
		now := time.Now()

		rpcReply := cm.passiveConsensusModule.rpc_RpcAppendEntries(from, rpc, now)

		select {
		case replyChan <- rpcReply:
		default:
			// theoretically unreachable as we make a buffered channel of
			// capacity 1 and this is the one send to it
			panic("FATAL: replyChan is nil or wants to block")
		}
	}
	return replyChan
}

// Process the given RpcRequestVote message from the given peer
// asynchronously.
//
// This method sends the RPC message to the ConsensusModule's goroutine.
// The RPC reply will be sent later on the returned channel.
//
// See RpcSender (in interfaces.go) for outgoing RPC.
//
// TODO: behavior when channel full?
func (cm *ConsensusModule) ProcessRpcRequestVoteAsync(
	from ServerId,
	rpc *RpcRequestVote,
) <-chan *RpcRequestVoteReply {
	replyChan := make(chan *RpcRequestVoteReply, 1)
	cm.runnableChannel <- func() {
		now := time.Now()

		rpcReply := cm.passiveConsensusModule.rpc_RpcRequestVote(from, rpc, now)

		select {
		case replyChan <- rpcReply:
		default:
			// theoretically unreachable as we make a buffered channel of
			// capacity 1 and this is the one send to it
			panic("FATAL: replyChan is nil or wants to block")
		}
	}
	return replyChan
}

// Append the given command as an entry in the log.
//
// This can only be done if the ConsensusModule is in LEADER state.
//
// The command should already have been validated by this point to ensure that
// it will succeed when applied to the state machine.
// (both internal contents and other context/state checks)
//
// This method sends the RPC message to the ConsensusModule's goroutine.
// The reply will be sent later on the returned channel when the append is
// processed. The reply will contain the index of the new entry or an error.
//
// Here, we intentionally punt on some of the leader details, specifically
// most of:
//
// #RFS-L2: If command received from client: append entry to local log,
// respond after entry applied to state machine (#5.3)
//
// We choose not to deal with the client directly. You must implement the
// interaction with clients and waiting the entry to be applied to the state
// machine. (see delegation of lastApplied to raft.Log)
//
// TODO: behavior when channel full?
func (cm *ConsensusModule) AppendCommandAsync(
	command Command,
) <-chan AppendCommandResult {
	replyChan := make(chan AppendCommandResult, 1)
	cm.runnableChannel <- func() {
		logIndex, err := cm.passiveConsensusModule.appendCommand(command)
		appendCommandResult := AppendCommandResult{logIndex, err}
		select {
		case replyChan <- appendCommandResult:
		default:
			// theoretically unreachable as we make a buffered channel of
			// capacity 1 and this is the one send to it
			panic("FATAL: replyChan is nil or wants to block")
		}
	}
	return replyChan
}

type AppendCommandResult struct {
	LogIndex
	error
}

// -- protected methods

// Implement rpcSender.sendRpcAppendEntriesAsync to bridge to
// RpcService.SendRpcAppendEntriesAsync() with a closure callback.
func (cm *ConsensusModule) sendRpcAppendEntriesAsync(toServer ServerId, rpc *RpcAppendEntries) {
	replyAsync := func(rpcReply *RpcAppendEntriesReply) {
		// Process the given RPC reply message from the given peer
		// asynchronously.
		// TODO: behavior when channel full?
		cm.runnableChannel <- func() {
			cm.passiveConsensusModule.rpcReply(toServer, rpc, rpcReply)
		}
	}
	cm.rpcService.SendRpcAppendEntriesAsync(toServer, rpc, replyAsync)
}

// Implement rpcSender.sendRpcRequestVoteAsync to bridge to
// RpcService.SendRpcRequestVoteAsync() with a closure callback.
func (cm *ConsensusModule) sendRpcRequestVoteAsync(toServer ServerId, rpc *RpcRequestVote) {
	replyAsync := func(rpcReply *RpcRequestVoteReply) {
		// Process the given RPC reply message from the given peer
		// asynchronously.
		// TODO: behavior when channel full?
		cm.runnableChannel <- func() {
			cm.passiveConsensusModule.rpcReply(toServer, rpc, rpcReply)
		}
	}
	cm.rpcService.SendRpcRequestVoteAsync(toServer, rpc, replyAsync)
}

func (cm *ConsensusModule) processor() {
	defer func() {
		// TODO: should we really recover?!
		// Recover & save the panic reason
		if r := recover(); r != nil {
			cm.stopError.Store(r)
		}
		// Mark the server as stopped
		atomic.StoreInt32(&cm.stopped, 1)
		// Clean up things
		close(cm.runnableChannel)
		cm.ticker.Stop()
	}()

loop:
	for {
		select {
		case runnable, ok := <-cm.runnableChannel:
			if !ok {
				// theoretically unreachable as we don't close the channel
				// til shutdown
				panic("FATAL: runnableChannel closed")
			}
			runnable()
		case _, ok := <-cm.ticker.C:
			if !ok {
				// theoretically unreachable as we don't stop the timer til shutdown
				panic("FATAL: ticker channel closed")
			}
			// Get a fresh now since the ticker's now could have been waiting
			now := time.Now()
			cm.passiveConsensusModule.tick(now)
		case <-cm.stopSignal:
			break loop
		}
	}
}

type rpcTuple struct {
	from      ServerId
	rpc       interface{}
	replyChan chan interface{}
}
