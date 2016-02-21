package raft

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"
)

const (
	RPC_CHANNEL_BUFFER_SIZE = 100
)

type passiveConsensusModule struct {
	// ===== the following fields meant to be immutable

	// -- External components
	persistentState PersistentState
	log             Log
	rpcSender       rpcSender

	// -- Config
	clusterInfo *ClusterInfo

	// ===== the following fields may be accessed concurrently

	// -- State - for all servers
	_unsafe_serverState ServerState

	// ===== the following fields meant for single-threaded access

	// -- State - for all servers
	// commitIndex is the index of highest log entry known to be committed
	// (initialized to 0, increases monotonically)
	_commitIndex           LogIndex
	electionTimeoutTracker *electionTimeoutTracker

	// -- State - for candidates only
	candidateVolatileState *candidateVolatileState

	// -- State - for leaders only
	leaderVolatileState *leaderVolatileState
}

func newPassiveConsensusModule(
	persistentState PersistentState,
	log Log,
	rpcSender rpcSender,
	clusterInfo *ClusterInfo,
	electionTimeoutLow time.Duration,
	now time.Time,
) *passiveConsensusModule {
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

	pcm := &passiveConsensusModule{
		// -- External components
		persistentState,
		log,
		rpcSender,

		// -- Config
		clusterInfo,

		// -- State - for all servers
		// #5.2-p1s2: When servers start up, they begin as followers
		FOLLOWER,

		// -- State - for all servers
		// commitIndex is the index of highest log entry known to be committed
		// (initialized to 0, increases monotonically)
		0,
		newElectionTimeoutTracker(electionTimeoutLow, now),

		// -- State - for candidates only
		nil,

		// -- State - for leaders only
		nil,
	}

	return pcm
}

// Get the current server state.
// Validates the server state before returning.
func (cm *passiveConsensusModule) getServerState() ServerState {
	serverState := ServerState(atomic.LoadUint32((*uint32)(&cm._unsafe_serverState)))
	validateServerState(serverState)
	return serverState
}

// Set the current server state.
// Validates the server state before setting.
func (cm *passiveConsensusModule) setServerState(serverState ServerState) {
	validateServerState(serverState)
	atomic.StoreUint32((*uint32)(&cm._unsafe_serverState), (uint32)(serverState))
}

// Get the current commitIndex value.
func (cm *passiveConsensusModule) getCommitIndex() LogIndex {
	return cm._commitIndex
}

// Set the current commitIndex value.
// Checks that it is does not reduce.
func (cm *passiveConsensusModule) setCommitIndex(commitIndex LogIndex) {
	if commitIndex < cm._commitIndex {
		panic(fmt.Sprintf(
			"setCommitIndex to %v < current commitIndex %v",
			commitIndex,
			cm._commitIndex,
		))
	}
	cm._commitIndex = commitIndex
	err := cm.log.CommitIndexChanged(commitIndex)
	if err != nil {
		panic(err)
	}
}

// Append the given command as an entry in the log.
// #RFS-L2a: If command received from client: append entry to local log
func (cm *passiveConsensusModule) appendCommand(
	command Command,
) (LogIndex, error) {
	serverState := cm.getServerState()
	if serverState == LEADER {
		iole, err := cm.log.GetIndexOfLastEntry()
		if err != nil {
			panic(err)
		}
		logEntries := []LogEntry{
			{cm.persistentState.GetCurrentTerm(), command},
		}
		err = cm.log.SetEntriesAfterIndex(iole, logEntries)
		if err != nil {
			panic(err)
		}
		var newIole LogIndex
		newIole, err = cm.log.GetIndexOfLastEntry()
		if err != nil {
			panic(err)
		}
		if newIole != iole+1 {
			panic(fmt.Sprintf("newIole=%v != %v + 1", newIole, iole))
		}
		return newIole, nil
	} else {
		return 0, errors.New("raft: state != LEADER - cannot append command to log")
	}
}

// Iterate
func (cm *passiveConsensusModule) tick(now time.Time) {
	serverState := cm.getServerState()
	switch serverState {
	case FOLLOWER:
		// #RFS-F2: If election timeout elapses without receiving
		// AppendEntries RPC from current leader or granting vote
		// to candidate: convert to candidate
		// #5.2-p1s5: If a follower receives no communication over a period
		// of time called the election timeout, then it assumes there is no
		// viable leader and begins an election to choose a new leader.
		fallthrough
	case CANDIDATE:
		// #RFS-C4: If election timeout elapses: start new election
		// #5.2-p5s1: The third possible outcome is that a candidate neither
		// wins nor loses the election; ... votes could be split so that no
		// candidate obtains a majority.
		// #5.2-p5s2: When this happens, each candidate will time out and
		// start a new election by incrementing its term and initiating
		// another round of RequestVote RPCs.
		if cm.electionTimeoutTracker.electionTimeoutHasOccurred(now) {
			cm.becomeCandidateAndBeginElection(now)
		}
	case LEADER:
		// #RFS-L4: If there exists an N such that N > commitIndex, a majority
		// of matchIndex[i] >= N, and log[N].term == currentTerm:
		// set commitIndex = N (#5.3, #5.4)
		cm.advanceCommitIndexIfPossible()
		// #RFS-L3.0: If last log index >= nextIndex for a follower: send
		// AppendEntries RPC with log entries starting at nextIndex
		cm.sendAppendEntriesToAllPeers(false)
	}
}

func (cm *passiveConsensusModule) becomeCandidateAndBeginElection(now time.Time) {
	// #RFS-C1: On conversion to candidate, start election:
	// Increment currentTerm; Vote for self; Send RequestVote RPCs
	// to all other servers; Reset election timer
	// #5.2-p2s1: To begin an election, a follower increments its
	// current term and transitions to candidate state.
	newTerm := cm.persistentState.GetCurrentTerm() + 1
	cm.persistentState.SetCurrentTerm(newTerm)
	cm.candidateVolatileState = newCandidateVolatileState(cm.clusterInfo)
	cm.setServerState(CANDIDATE)
	// #5.2-p2s2: It then votes for itself and issues RequestVote RPCs
	// in parallel to each of the other servers in the cluster.
	cm.persistentState.SetVotedFor(cm.clusterInfo.GetThisServerId())
	lastLogIndex, lastLogTerm := GetIndexAndTermOfLastEntry(cm.log)
	cm.clusterInfo.ForEachPeer(
		func(serverId ServerId) {
			rpcRequestVote := &RpcRequestVote{newTerm, lastLogIndex, lastLogTerm}
			cm.rpcSender.sendRpcRequestVoteAsync(serverId, rpcRequestVote)
		},
	)
	// Reset election timeout!
	cm.electionTimeoutTracker.chooseNewRandomElectionTimeoutAndTouch(now)
}

func (cm *passiveConsensusModule) becomeLeader() {
	iole, err := cm.log.GetIndexOfLastEntry()
	if err != nil {
		panic(err)
	}
	cm.leaderVolatileState = newLeaderVolatileState(cm.clusterInfo, iole)
	cm.setServerState(LEADER)
	// #RFS-L1a: Upon election: send initial empty AppendEntries RPCs (heartbeat)
	// to each server;
	cm.sendAppendEntriesToAllPeers(true)
}

func (cm *passiveConsensusModule) becomeFollowerWithTerm(newTerm TermNo) {
	cm.setServerState(FOLLOWER)
	cm.persistentState.SetCurrentTerm(newTerm)
}

// -- leader code

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
		var err error
		peerLastLogTerm, err = cm.log.GetTermAtIndex(peerLastLogIndex)
		if err != nil {
			panic(err)
		}
	}
	//
	var entriesToSend []LogEntry
	if empty {
		entriesToSend = []LogEntry{}
	} else {
		var err error
		entriesToSend, err = cm.log.GetEntriesAfterIndex(peerLastLogIndex)
		if err != nil {
			panic(err)
		}
	}
	//
	rpcAppendEntries := &RpcAppendEntries{
		serverTerm,
		peerLastLogIndex,
		peerLastLogTerm,
		entriesToSend,
		cm.getCommitIndex(),
	}
	cm.rpcSender.sendRpcAppendEntriesAsync(peerId, rpcAppendEntries)
}

// #RFS-L4: If there exists an N such that N > commitIndex, a majority
// of matchIndex[i] >= N, and log[N].term == currentTerm:
// set commitIndex = N (#5.3, #5.4)
func (cm *passiveConsensusModule) advanceCommitIndexIfPossible() {
	newerCommitIndex := findNewerCommitIndex(
		cm.clusterInfo,
		cm.leaderVolatileState,
		cm.log,
		cm.persistentState.GetCurrentTerm(),
		cm.getCommitIndex(),
	)
	if newerCommitIndex != 0 && newerCommitIndex > cm.getCommitIndex() {
		cm.setCommitIndex(newerCommitIndex)
	}
}

// -- rpc bridging things

// This is an internal equivalent to RpcService without the reply setup.
type rpcSender interface {
	sendRpcAppendEntriesAsync(toServer ServerId, rpc *RpcAppendEntries)
	sendRpcRequestVoteAsync(toServer ServerId, rpc *RpcRequestVote)
}
