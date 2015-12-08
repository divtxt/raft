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
	_unsafe_serverState ServerState

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
		volatileState{}, // TODO: do these values need to be initialized?
		now,             // temp value, to be replaced before goroutine start
		nil,
	}

	pcm.chooseNewRandomElectionTimeout()
	pcm.resetElectionTimeoutTime(now)

	return pcm, now
}

// Get the current server state.
// Validates the server state before returning.
func (cm *passiveConsensusModule) getServerState() ServerState {
	serverState := ServerState(atomic.LoadUint32((*uint32)(&cm._unsafe_serverState)))
	_validateServerState(serverState)
	return serverState
}

// Set the current server state.
// Validates the server state before setting.
func (cm *passiveConsensusModule) _setServerState(serverState ServerState) {
	_validateServerState(serverState)
	atomic.StoreUint32((*uint32)(&cm._unsafe_serverState), (uint32)(serverState))
}

func (cm *passiveConsensusModule) chooseNewRandomElectionTimeout() {
	// #5.2-p6s2: ..., election timeouts are chosen randomly from a fixed
	// interval (e.g., 150-300ms)
	cm.currentElectionTimeout = cm.electionTimeoutLow + time.Duration(rand.Int63n(int64(cm.electionTimeoutLow)+1))
}

func (cm *passiveConsensusModule) resetElectionTimeoutTime(now time.Time) {
	cm.electionTimeoutTime = now.Add(cm.currentElectionTimeout)
}

// Process the given rpc message
func (cm *passiveConsensusModule) rpc(
	from ServerId,
	rpc interface{},
) interface{} {
	serverState := cm.getServerState()

	switch rpc := rpc.(type) {
	case *RpcAppendEntries:
		success := cm._processRpc_AppendEntries(serverState, from, rpc)
		rpcReply := &RpcAppendEntriesReply{
			cm.persistentState.GetCurrentTerm(),
			success,
		}
		return rpcReply
	case *RpcRequestVote:
		success := cm._processRpc_RequestVote(serverState, from, rpc)
		rpcReply := &RpcRequestVoteReply{
			success,
		}
		return rpcReply
	default:
		panic(fmt.Sprintf("FATAL: unknown rpc type: %T from: %v", rpc, from))
	}
}

func (cm *passiveConsensusModule) rpcReply(
	from ServerId,
	rpcReply interface{},
) {
	serverState := cm.getServerState()

	switch rpcReply := rpcReply.(type) {
	case *RpcAppendEntriesReply:
		cm._processRpc_AppendEntriesReply(serverState, from, rpcReply)
	case *RpcRequestVoteReply:
		cm._processRpc_RequestVoteReply(serverState, from, rpcReply)
	default:
		panic(fmt.Sprintf("FATAL: unknown rpcReply type: %T from: %v", rpcReply, from))
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
		fallthrough
	case CANDIDATE:
		// #5.2-p5s1: The third possible outcome is that a candidate neither
		// wins nor loses the election; ... votes could be split so that no
		// candidate obtains a majority.
		// #5.2-p5s2: When this happens, each candidate will time out and
		// start a new election by incrementing its term and initiating
		// another round of RequestVote RPCs.
		if now.After(cm.electionTimeoutTime) {
			cm.becomeCandidateAndBeginElection(now)
		}
		// TODO: else/and anything else?
	case LEADER:
		cm._sendEmptyAppendEntriesToAllPeers()
		// TODO: more leader things
	}
}

func (cm *passiveConsensusModule) becomeCandidateAndBeginElection(now time.Time) {
	// #5.2-p2s1: To begin an election, a follower increments its
	// current term and transitions to candidate state.
	newTerm := cm.persistentState.GetCurrentTerm() + 1
	cm.candidateVolatileState = newCandidateVolatileState(cm.peerServerIds)
	cm._setServerState(CANDIDATE)
	// #5.2-p2s2: It then votes for itself and issues RequestVote RPCs
	// in parallel to each of the other servers in the cluster.
	cm.persistentState.SetCurrentTermAndVotedFor(newTerm, cm.thisServerId)
	lastLogIndex, lastLogTerm := getIndexAndTermOfLastEntry(cm.log)
	for _, serverId := range cm.peerServerIds {
		rpcRequestVote := &RpcRequestVote{newTerm, lastLogIndex, lastLogTerm}
		cm.rpcSender.SendAsync(serverId, rpcRequestVote)
	}
	// Reset election timeout!
	cm.chooseNewRandomElectionTimeout()
	cm.resetElectionTimeoutTime(now)
}

func (cm *passiveConsensusModule) becomeLeader() {
	cm._setServerState(LEADER)
	// #RFS-L1a: Upon election: send initial empty AppendEntries RPCs (heartbeat)
	// to each server;
	cm._sendEmptyAppendEntriesToAllPeers()
	// TODO: more leader things!
}

func (cm *passiveConsensusModule) becomeFollower() {
	cm._setServerState(FOLLOWER)
	// TODO: more follower things!
}

// leader code
func (cm *passiveConsensusModule) _sendEmptyAppendEntriesToAllPeers() {
	serverTerm := cm.persistentState.GetCurrentTerm()
	lastLogIndex, lastLogTerm := getIndexAndTermOfLastEntry(cm.log)
	rpcAppendEntries := &RpcAppendEntries{
		serverTerm,
		lastLogIndex,
		lastLogTerm,
		[]LogEntry{},
		cm.volatileState.commitIndex,
	}
	for _, serverId := range cm.peerServerIds {
		cm.rpcSender.SendAsync(serverId, rpcAppendEntries)
	}
}
