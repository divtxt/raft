package raft

import (
	"fmt"
	"sync/atomic"
	"time"
)

const (
	RPC_CHANNEL_BUFFER_SIZE = 100
)

type passiveConsensusModule struct {
	// -- External components - these fields meant to be immutable
	persistentState PersistentState
	log             Log
	rpcSender       rpcSender

	// -- Config - these fields meant to be immutable
	clusterInfo              *ClusterInfo
	maxEntriesPerAppendEntry uint64

	// -- State - these fields may be accessed concurrently
	_unsafe_serverState ServerState

	// -- State - these fields meant for single-threaded access
	volatileState          volatileState
	electionTimeoutTracker *electionTimeoutTracker
	candidateVolatileState *candidateVolatileState
	leaderVolatileState    *leaderVolatileState
}

func newPassiveConsensusModule(
	persistentState PersistentState,
	log Log,
	rpcSender rpcSender,
	clusterInfo *ClusterInfo,
	electionTimeoutLow time.Duration,
	maxEntriesPerAppendEntry uint64,
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
	if clusterInfo == nil {
		panic("clusterInfo cannot be nil")
	}
	if electionTimeoutLow.Nanoseconds() <= 0 {
		panic("electionTimeoutLow must be greater than zero")
	}

	now := time.Now()

	pcm := &passiveConsensusModule{
		// -- External components
		persistentState,
		log,
		rpcSender,

		// -- Config
		clusterInfo,
		maxEntriesPerAppendEntry,

		// -- State
		// #5.2-p1s2: When servers start up, they begin as followers
		FOLLOWER,

		// -- State
		volatileState{},
		newElectionTimeoutTracker(electionTimeoutLow, now),
		nil,
		nil,
	}

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

// Process the given rpc message
func (cm *passiveConsensusModule) rpc(
	from ServerId,
	rpc interface{},
	now time.Time,
) interface{} {
	serverState := cm.getServerState()

	switch rpc := rpc.(type) {
	case *RpcAppendEntries:
		success := cm._processRpc_AppendEntries(serverState, from, rpc, now)
		rpcReply := &RpcAppendEntriesReply{
			cm.persistentState.GetCurrentTerm(),
			success,
		}
		return rpcReply
	case *RpcRequestVote:
		voteGranted := cm._processRpc_RequestVote(serverState, from, rpc, now)
		rpcReply := &RpcRequestVoteReply{
			cm.persistentState.GetCurrentTerm(),
			voteGranted,
		}
		return rpcReply
	default:
		panic(fmt.Sprintf("FATAL: unknown rpc type: %T from: %v", rpc, from))
	}
}

func (cm *passiveConsensusModule) rpcReply(
	from ServerId,
	rpc interface{},
	rpcReply interface{},
) {
	serverState := cm.getServerState()

	switch rpc := rpc.(type) {
	case *RpcAppendEntries:
		switch rpcReply := rpcReply.(type) {
		case *RpcAppendEntriesReply:
			cm._processRpc_AppendEntriesReply(serverState, from, rpc, rpcReply)
		default:
			panic(fmt.Sprintf("FATAL: mismatched rpcReply type: %T from: %v - expected *raft.RpcAppendEntriesReply", rpcReply, from))
		}
	case *RpcRequestVote:
		switch rpcReply := rpcReply.(type) {
		case *RpcRequestVoteReply:
			cm._processRpc_RequestVoteReply(serverState, from, rpc, rpcReply)
		default:
			panic(fmt.Sprintf("FATAL: mismatched rpcReply type: %T from: %v - expected *raft.RpcRequestVoteReply", rpcReply, from))
		}
	default:
		panic(fmt.Sprintf("FATAL: unknown rpc type: %T from: %v", rpc, from))
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
		if cm.electionTimeoutTracker.electionTimeoutHasOccurred(now) {
			cm.becomeCandidateAndBeginElection(now)
		}
		// TODO: else/and anything else?
	case LEADER:
		// #RFS-L3.0: If last log index >= nextIndex for a follower: send
		// AppendEntries RPC with log entries starting at nextIndex
		cm.sendAppendEntriesToAllPeers(false)
		// #RFS-L4: If there exists an N such that N > commitIndex, a majority
		// of matchIndex[i] >= N, and log[N].term == currentTerm:
		// set commitIndex = N (#5.3, #5.4)
		cm.advanceCommitIndexIfPossible()
		// TODO: more leader things
	}
}

func (cm *passiveConsensusModule) becomeCandidateAndBeginElection(now time.Time) {
	// #5.2-p2s1: To begin an election, a follower increments its
	// current term and transitions to candidate state.
	newTerm := cm.persistentState.GetCurrentTerm() + 1
	cm.persistentState.SetCurrentTerm(newTerm)
	cm.candidateVolatileState = newCandidateVolatileState(cm.clusterInfo)
	cm._setServerState(CANDIDATE)
	// #5.2-p2s2: It then votes for itself and issues RequestVote RPCs
	// in parallel to each of the other servers in the cluster.
	cm.persistentState.SetVotedFor(cm.clusterInfo.GetThisServerId())
	lastLogIndex, lastLogTerm := GetIndexAndTermOfLastEntry(cm.log)
	cm.clusterInfo.ForEachPeer(
		func(serverId ServerId) {
			rpcRequestVote := &RpcRequestVote{newTerm, lastLogIndex, lastLogTerm}
			cm.rpcSender.sendAsync(serverId, rpcRequestVote)
		},
	)
	// Reset election timeout!
	cm.electionTimeoutTracker.chooseNewRandomElectionTimeoutAndTouch(now)
}

func (cm *passiveConsensusModule) becomeLeader() {
	cm.leaderVolatileState = newLeaderVolatileState(cm.clusterInfo, cm.log.GetIndexOfLastEntry())
	cm._setServerState(LEADER)
	// #RFS-L1a: Upon election: send initial empty AppendEntries RPCs (heartbeat)
	// to each server;
	cm.sendAppendEntriesToAllPeers(true)
	// TODO: more leader things!
}

func (cm *passiveConsensusModule) becomeFollowerWithTerm(newTerm TermNo) {
	cm._setServerState(FOLLOWER)
	cm.persistentState.SetCurrentTerm(newTerm)
	// TODO: more follower things!
}

// -- leader code

func (cm *passiveConsensusModule) _sendEmptyAppendEntriesToAllPeers() {
	serverTerm := cm.persistentState.GetCurrentTerm()
	lastLogIndex, lastLogTerm := GetIndexAndTermOfLastEntry(cm.log)
	rpcAppendEntries := &RpcAppendEntries{
		serverTerm,
		lastLogIndex,
		lastLogTerm,
		[]LogEntry{},
		cm.volatileState.commitIndex,
	}
	cm.clusterInfo.ForEachPeer(
		func(serverId ServerId) {
			cm.rpcSender.sendAsync(serverId, rpcAppendEntries)
		},
	)
}

func (cm *passiveConsensusModule) sendAppendEntriesToAllPeers(empty bool) {
	cm.clusterInfo.ForEachPeer(
		func(serverId ServerId) {
			cm.sendAppendEntriesToPeer(serverId, empty)
		},
	)
}

func (cm *passiveConsensusModule) sendAppendEntriesToPeer(
	peerId ServerId,
	empty bool,
) {
	serverTerm := cm.persistentState.GetCurrentTerm()
	//
	peerLastLogIndex := cm.leaderVolatileState.getNextIndex(peerId) - 1
	var peerLastLogTerm TermNo
	if peerLastLogIndex == 0 {
		peerLastLogTerm = 0
	} else {
		peerLastLogTerm = cm.log.GetTermAtIndex(peerLastLogIndex)
	}
	//
	var entriesToSend []LogEntry
	if empty {
		entriesToSend = []LogEntry{}
	} else {
		entriesToSend = cm.getEntriesAfterLogIndex(peerLastLogIndex)
	}
	//
	rpcAppendEntries := &RpcAppendEntries{
		serverTerm,
		peerLastLogIndex,
		peerLastLogTerm,
		entriesToSend,
		0, // TODO: cm.volatileState.commitIndex
	}
	cm.rpcSender.sendAsync(peerId, rpcAppendEntries)
}

func (cm *passiveConsensusModule) getEntriesAfterLogIndex(afterLogIndex LogIndex) []LogEntry {
	indexOfLastEntry := cm.log.GetIndexOfLastEntry()

	if indexOfLastEntry < afterLogIndex {
		panic(fmt.Sprintf(
			"indexOfLastEntry=%v is < afterLogIndex=%v",
			afterLogIndex,
			indexOfLastEntry,
		))
	}

	var numEntriesToGet uint64 = uint64(indexOfLastEntry - afterLogIndex)

	// Short-circuit allocation for common case
	if numEntriesToGet == 0 {
		return []LogEntry{}
	}

	if numEntriesToGet > cm.maxEntriesPerAppendEntry {
		numEntriesToGet = cm.maxEntriesPerAppendEntry
	}

	logEntries := make([]LogEntry, numEntriesToGet)
	var i uint64 = 0
	nextIndexToGet := afterLogIndex + 1

	for i < numEntriesToGet {
		logEntries[i] = cm.log.GetLogEntryAtIndex(nextIndexToGet)
		i++
		nextIndexToGet++
	}

	return logEntries
}

func (cm *passiveConsensusModule) advanceCommitIndexIfPossible() {
	newerCommitIndex := findNewerCommitIndex(
		cm.clusterInfo,
		cm.leaderVolatileState,
		cm.log,
		cm.persistentState.GetCurrentTerm(),
		cm.volatileState.commitIndex,
	)
	if newerCommitIndex != 0 && newerCommitIndex > cm.volatileState.commitIndex {
		cm.volatileState.commitIndex = newerCommitIndex
	}
}

// -- rpc bridging things

// This is an internal equivalent to RpcService without the reply setup.
type rpcSender interface {
	sendAsync(toServer ServerId, rpc interface{})
}
