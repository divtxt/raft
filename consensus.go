package raft

import (
	"fmt"
	"math/rand"
	"sync/atomic"
	"time"
)

type TimeSettings struct {
	TickerDuration time.Duration

	// Election timeout low value - 2x this value is used as high value.
	ElectionTimeoutLow time.Duration
}

const (
	RPC_CHANNEL_BUFFER_SIZE = 100
)

type passiveConsensusModule struct {
	// -- External components - these fields meant to be immutable
	persistentState PersistentState
	log             Log
	rpcSender       RpcSender

	// -- Config - these fields meant to be immutable
	thisServerId           ServerId
	peerServerIds          []ServerId
	electionTimeoutLow     time.Duration
	currentElectionTimeout time.Duration

	// -- State - these fields may be accessed concurrently
	serverState ServerState

	// -- State - these fields meant for single-threaded access
	volatileState          volatileState
	electionTimeoutTime    time.Time
	candidateVolatileState *candidateVolatileState
}

func newPassiveConsensusModule(
	persistentState PersistentState,
	log Log,
	rpcSender RpcSender,
	thisServerId ServerId,
	peerServerIds []ServerId,
	timeSettings TimeSettings,
) (*passiveConsensusModule, time.Time) {
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

	now := time.Now()

	pcm := &passiveConsensusModule{
		// -- External components
		persistentState,
		log,
		rpcSender,

		// -- Config
		thisServerId,
		peerServerIds,
		timeSettings.ElectionTimeoutLow,
		0, // temp value, to be replaced before goroutine start

		// -- State
		// #5.2-p1s2: When servers start up, they begin as followers
		FOLLOWER,

		// -- State
		volatileState{},
		now, // temp value, to be replaced before goroutine start
		nil,
	}

	pcm.chooseNewRandomElectionTimeout()
	pcm.resetElectionTimeoutTime(now)

	return pcm, now
}

// Get the current server state
func (cm *passiveConsensusModule) getServerState() ServerState {
	return ServerState(atomic.LoadUint32((*uint32)(&cm.serverState)))
}

// Set the current server state
func (cm *passiveConsensusModule) setServerState(serverState ServerState) {
	if serverState != FOLLOWER && serverState != CANDIDATE && serverState != LEADER {
		panic(fmt.Sprintf("FATAL: unknown ServerState: %v", serverState))
	}
	atomic.StoreUint32((*uint32)(&cm.serverState), (uint32)(serverState))
}

func (cm *passiveConsensusModule) chooseNewRandomElectionTimeout() {
	// #5.2-p6s2: ..., election timeouts are chosen randomly from a fixed
	// interval (e.g., 150-300ms)
	cm.currentElectionTimeout = cm.electionTimeoutLow + time.Duration(rand.Int63n(int64(cm.electionTimeoutLow)+1))
}

func (cm *passiveConsensusModule) resetElectionTimeoutTime(now time.Time) {
	cm.electionTimeoutTime = now.Add(cm.currentElectionTimeout)
}

type rpcTuple struct {
	from ServerId
	rpc  interface{}
}

// Process the given rpc message
func (cm *passiveConsensusModule) rpc(from ServerId, rpc interface{}) {
	switch rpc := rpc.(type) {

	case *RpcAppendEntries:
		success := cm._processRpc_AppendEntries(rpc)
		reply := &RpcAppendEntriesReply{
			cm.persistentState.GetCurrentTerm(),
			success,
		}
		cm.rpcSender.SendAsync(from, reply)
	case *RpcRequestVote:
		success := cm._processRpc_RequestVote(from, rpc)
		reply := &RpcRequestVoteReply{
			success,
		}
		cm.rpcSender.SendAsync(from, reply)
	case *RpcRequestVoteReply:
		cm._processRpc_RequestVoteReply(from, rpc)
	default:
		panic(fmt.Sprintf("FATAL: unknown rpc type: %T", rpc))
	}
}

// Iterate
func (cm *passiveConsensusModule) tick(now time.Time) {
	serverState := cm.getServerState()
	switch serverState {
	case FOLLOWER:
		// #5.2-p1s5: If a follower receives no communication over a period
		// of time called the election timeout, then it assumes there is no
		// viable leader and begins an election to choose a new leader.
		if now.After(cm.electionTimeoutTime) {
			cm.beginElection(now)
		}
	case CANDIDATE:
		// #5.2-p5s1: The third possible outcome is that a candidate neither
		// wins nor loses the election; ... votes could be split so that no
		// candidate obtains a majority.
		// #5.2-p5s2: When this happens, each candidate will time out and
		// start a new election by incrementing its term and initiating
		// another round of RequestVote RPCs.
		if now.After(cm.electionTimeoutTime) {
			cm.beginElection(now)
		}
		// TODO: else/and anything else?
	case LEADER:
		// FIXME
		panic("todo: LEADER tick()")
	default:
		panic(fmt.Sprintf("FATAL: unknown ServerState: %v", serverState))
	}
}

func (cm *passiveConsensusModule) beginElection(now time.Time) {
	// #5.2-p2s1: To begin an election, a follower increments its
	// current term and transitions to candidate state.
	newTerm := cm.persistentState.GetCurrentTerm() + 1
	cm.candidateVolatileState = newCandidateVolatileState(cm.peerServerIds)
	cm.setServerState(CANDIDATE)
	// #5.2-p2s2: It then votes for itself and issues RequestVote RPCs
	// in parallel to each of the other servers in the cluster.
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
	// Reset election timeout!
	cm.chooseNewRandomElectionTimeout()
	cm.resetElectionTimeoutTime(now)
}
