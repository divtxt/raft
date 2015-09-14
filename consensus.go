package raft

import (
	"fmt"
	"sync/atomic"
	"time"
)

type TimeSettings struct {
	tickerDuration  time.Duration
	electionTimeout time.Duration
}

const (
	RPC_CHANNEL_BUFFER_SIZE = 100
)

type ConsensusModule struct {
	// -- External components - these fields meant to be immutable
	persistentState PersistentState
	log             Log
	rpcSender       RpcSender

	// -- Config - these fields meant to be immutable
	thisServerId  ServerId
	peerServerIds []ServerId
	timeSettings  TimeSettings

	// -- State - these fields may be accessed concurrently
	stopped     int32
	serverState ServerState

	// -- State - these fields meant for use within the goroutine
	volatileState          VolatileState
	electionTimeoutTime    time.Time
	candidateVolatileState *CandidateVolatileState

	// -- Channels
	rpcChannel chan rpcTuple
	ticker     *time.Ticker

	// -- Control
	stopSignal chan struct{}
	stopError  interface{}
}

// Initialize a consensus module with the given components and settings.
// A goroutine that handles consensus processing is created.
// All parameters are required and cannot be nil.
// Server ids are check using ValidateServerIds().
// Time settings is checked using ValidateTimeSettings().
func NewConsensusModule(
	persistentState PersistentState,
	log Log,
	rpcSender RpcSender,
	thisServerId ServerId,
	peerServerIds []ServerId,
	timeSettings TimeSettings,
) *ConsensusModule {
	// Param checks
	if persistentState == nil {
		panic("'persistentState' cannot be nil")
	}
	if log == nil {
		panic("'log' cannot be nil")
	}
	if rpcSender == nil {
		panic("'rpcSender' cannot be nil")
	}
	if emsg := ValidateServerIds(thisServerId, peerServerIds); emsg != "" {
		panic(emsg)
	}
	if emsg := ValidateTimeSettings(timeSettings); emsg != "" {
		panic(emsg)
	}

	rpcChannel := make(chan rpcTuple, RPC_CHANNEL_BUFFER_SIZE)
	ticker := time.NewTicker(timeSettings.tickerDuration)

	cm := &ConsensusModule{
		// -- External components
		persistentState,
		log,
		rpcSender,

		// -- Config
		thisServerId,
		peerServerIds,
		timeSettings,

		// -- State
		0,
		// #5.2-p1s2: When servers start up, they begin as followers
		FOLLOWER,

		// -- State
		VolatileState{},
		time.Now(), // temp value, to be replaced before goroutine start
		nil,

		// -- Channels
		rpcChannel,
		ticker,

		// -- Control
		make(chan struct{}),
		nil,
	}

	cm.resetElectionTimeoutTime()

	// Start the go routine
	go cm.processor()

	return cm
}

// Check if the goroutine is stopped.
func (cm *ConsensusModule) IsStopped() bool {
	return atomic.LoadInt32(&cm.stopped) != 0
}

// Stop the consensus module asynchronously.
// This will stop the goroutine that does the processing.
// Safe to call even if the goroutine has stopped.
// Will panic if called more than once.
func (cm *ConsensusModule) StopAsync() {
	close(cm.stopSignal)
}

// Get the panic error value that stopped the goroutine.
// The value will be nil if the goroutine is not stopped, or stopped
// without an error, or  panicked with a nil value.
func (cm *ConsensusModule) GetStopError() interface{} {
	return cm.stopError
}

// Get the current server state
func (cm *ConsensusModule) GetServerState() ServerState {
	return ServerState(atomic.LoadUint32((*uint32)(&cm.serverState)))
}

// Process the given RPC
func (cm *ConsensusModule) ProcessRpc(appendEntries AppendEntries) (AppendEntriesReply, error) {
	success, err := cm._processRpc_AppendEntries(appendEntries)
	return AppendEntriesReply{cm.persistentState.GetCurrentTerm(), success}, err
}

// Process the given RPC message from the given peer asynchronously.
// This method sends the rpc to the ConsensusModule's goroutine.
// Sending an unknown or unexpected rpc message will cause the
// ConsensusModule goroutine to panic and stop.
func (cm *ConsensusModule) ProcessRpcAsync(from ServerId, rpc interface{}) {
	select {
	case cm.rpcChannel <- rpcTuple{from, rpc}:
	default:
		// FIXME
		panic("oops!")
	}
}

type rpcTuple struct {
	from ServerId
	rpc  interface{}
}

// -- protected methods

// Set the current server state
func (cm *ConsensusModule) setServerState(serverState ServerState) {
	if serverState != FOLLOWER && serverState != CANDIDATE && serverState != LEADER {
		panic(fmt.Sprintf("FATAL: unknown ServerState: %v", serverState))
	}
	atomic.StoreUint32((*uint32)(&cm.serverState), (uint32)(serverState))
}

func (cm *ConsensusModule) resetElectionTimeoutTime() {
	cm.electionTimeoutTime = time.Now().Add(cm.timeSettings.electionTimeout)
}

func (cm *ConsensusModule) processor() {
	defer func() {
		// Recover & save the panic reason
		if r := recover(); r != nil {
			cm.stopError = r
		}
		// Mark the server as stopped
		atomic.StoreInt32(&cm.stopped, 1)
		// Clean up things
		close(cm.rpcChannel)
		cm.ticker.Stop()
		// TODO: call stop event listener(s)
	}()

loop:
	for {
		select {
		case rpc, ok := <-cm.rpcChannel:
			if !ok {
				panic("FATAL: rpcChannel closed")
			}
			cm.rpc(rpc.from, rpc.rpc)
		case now, ok := <-cm.ticker.C:
			if !ok {
				panic("FATAL: ticker channel closed")
			}
			cm.tick(now)
		case <-cm.stopSignal:
			break loop
		}
	}
}

func (cm *ConsensusModule) rpc(from ServerId, rpc interface{}) {
	switch rpc := rpc.(type) {
	case *RpcRequestVoteReply:
		cm._processRpc_RequestVoteReply(from, rpc)
	default:
		panic(fmt.Sprintf("FATAL: unknown rpc type: %T", rpc))
	}
}

func (cm *ConsensusModule) tick(now time.Time) {

	serverState := cm.GetServerState()
	switch serverState {
	case FOLLOWER:
		// #5.2-p1s5: If a follower receives no communication over a period
		// of time called the election timeout, then it assumes there is no
		// viable leader and begins an election to choose a new leader.
		if now.After(cm.electionTimeoutTime) {
			// #5.2-p2s1: To begin an election, a follower increments its
			// current term and transitions to candidate state.
			// #5.2-p2s2: It then votes for itself and issues RequestVote RPCs
			// in parallel to each of the other servers in the cluster.
			newTerm := cm.persistentState.GetCurrentTerm() + 1
			cm.candidateVolatileState = newCandidateVolatileState(cm.peerServerIds)
			cm.setServerState(CANDIDATE)
			cm.persistentState.SetCurrentTermAndVotedFor(newTerm, cm.thisServerId)
			lastLogIndex := cm.log.getIndexOfLastEntry()
			var lastLogTerm TermNo
			if lastLogIndex > 0 {
				lastLogTerm = cm.log.getTermAtIndex(lastLogIndex)
			} else {
				lastLogTerm = 0
			}
			for _, serverId := range cm.peerServerIds {
				rpcRequestVote := &RpcRequestVote{newTerm, lastLogIndex, lastLogTerm}
				cm.rpcSender.SendAsync(serverId, rpcRequestVote)
			}
		}
	case CANDIDATE:
		// FIXME
		panic("todo")
	case LEADER:
		// FIXME
		panic("todo")
	default:
		panic(fmt.Sprintf("FATAL: unknown ServerState: %v", serverState))
	}

}
