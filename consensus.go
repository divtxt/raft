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
) (*passiveConsensusModule, error) {
	// Param checks
	if persistentState == nil {
		return nil, errors.New("'persistentState' cannot be nil")
	}
	if log == nil {
		return nil, errors.New("'log' cannot be nil")
	}
	if rpcSender == nil {
		return nil, errors.New("'rpcSender' cannot be nil")
	}
	if clusterInfo == nil {
		return nil, errors.New("clusterInfo cannot be nil")
	}
	if electionTimeoutLow.Nanoseconds() <= 0 {
		return nil, errors.New("electionTimeoutLow must be greater than zero")
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

	return pcm, nil
}

// Get the current server state.
// Validates the server state before returning.
func (cm *passiveConsensusModule) getServerState() ServerState {
	serverState := ServerState(atomic.LoadUint32((*uint32)(&cm._unsafe_serverState)))
	err := validateServerState(serverState)
	if err != nil {
		panic(err.Error())
	}
	return serverState
}

// Set the current server state.
// Validates the server state before setting.
func (cm *passiveConsensusModule) setServerState(serverState ServerState) {
	err := validateServerState(serverState)
	if err != nil {
		panic(err.Error())
	}
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
			return 0, err
		}
		logEntries := []LogEntry{
			{cm.persistentState.GetCurrentTerm(), command},
		}
		err = cm.log.SetEntriesAfterIndex(iole, logEntries)
		if err != nil {
			return 0, err
		}
		var newIole LogIndex
		newIole, err = cm.log.GetIndexOfLastEntry()
		if err != nil {
			return 0, err
		}
		if newIole != iole+1 {
			return 0, fmt.Errorf("newIole=%v != %v + 1", newIole, iole)
		}
		return newIole, nil
	} else {
		return 0, errors.New("raft: state != LEADER - cannot append command to log")
	}
}

// Iterate
func (cm *passiveConsensusModule) tick(now time.Time) error {
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
			err := cm.becomeCandidateAndBeginElection(now)
			if err != nil {
				return err
			}
		}
	case LEADER:
		// #RFS-L4: If there exists an N such that N > commitIndex, a majority
		// of matchIndex[i] >= N, and log[N].term == currentTerm:
		// set commitIndex = N (#5.3, #5.4)
		err := cm.advanceCommitIndexIfPossible()
		if err != nil {
			return err
		}
		// #RFS-L3.0: If last log index >= nextIndex for a follower: send
		// AppendEntries RPC with log entries starting at nextIndex
		err = cm.sendAppendEntriesToAllPeers(false)
		if err != nil {
			return err
		}
	}
	return nil
}

func (cm *passiveConsensusModule) becomeCandidateAndBeginElection(now time.Time) error {
	// #RFS-C1: On conversion to candidate, start election:
	// Increment currentTerm; Vote for self; Send RequestVote RPCs
	// to all other servers; Reset election timer
	// #5.2-p2s1: To begin an election, a follower increments its
	// current term and transitions to candidate state.
	newTerm := cm.persistentState.GetCurrentTerm() + 1
	err := cm.persistentState.SetCurrentTerm(newTerm)
	if err != nil {
		return err
	}
	cm.candidateVolatileState, err = newCandidateVolatileState(cm.clusterInfo)
	if err != nil {
		return err
	}
	cm.setServerState(CANDIDATE)
	// #5.2-p2s2: It then votes for itself and issues RequestVote RPCs
	// in parallel to each of the other servers in the cluster.
	err = cm.persistentState.SetVotedFor(cm.clusterInfo.GetThisServerId())
	if err != nil {
		return err
	}
	lastLogIndex, lastLogTerm, err := GetIndexAndTermOfLastEntry(cm.log)
	if err != nil {
		return err
	}
	err = cm.clusterInfo.ForEachPeer(
		func(serverId ServerId) error {
			rpcRequestVote := &RpcRequestVote{newTerm, lastLogIndex, lastLogTerm}
			cm.rpcSender.sendRpcRequestVoteAsync(serverId, rpcRequestVote)
			return nil
		},
	)
	if err != nil {
		return err
	}
	// Reset election timeout!
	cm.electionTimeoutTracker.chooseNewRandomElectionTimeoutAndTouch(now)
	return nil
}

func (cm *passiveConsensusModule) becomeLeader() error {
	iole, err := cm.log.GetIndexOfLastEntry()
	if err != nil {
		return err
	}
	cm.leaderVolatileState, err = newLeaderVolatileState(cm.clusterInfo, iole)
	if err != nil {
		return err
	}
	cm.setServerState(LEADER)
	// #RFS-L1a: Upon election: send initial empty AppendEntries RPCs (heartbeat)
	// to each server;
	err = cm.sendAppendEntriesToAllPeers(true)
	if err != nil {
		return err
	}
	return nil
}

func (cm *passiveConsensusModule) becomeFollowerWithTerm(newTerm TermNo) error {
	cm.setServerState(FOLLOWER)
	err := cm.persistentState.SetCurrentTerm(newTerm)
	if err != nil {
		return err
	}
	return nil
}

// -- leader code

func (cm *passiveConsensusModule) sendAppendEntriesToAllPeers(empty bool) error {
	return cm.clusterInfo.ForEachPeer(
		func(serverId ServerId) error {
			err := cm.sendAppendEntriesToPeer(serverId, empty)
			if err != nil {
				return err
			}
			return nil
		},
	)
}

func (cm *passiveConsensusModule) sendAppendEntriesToPeer(
	peerId ServerId,
	empty bool,
) error {
	serverTerm := cm.persistentState.GetCurrentTerm()
	//
	peerNextIndex, err := cm.leaderVolatileState.getNextIndex(peerId)
	if err != nil {
		return err
	}
	peerLastLogIndex := peerNextIndex - 1
	var peerLastLogTerm TermNo
	if peerLastLogIndex == 0 {
		peerLastLogTerm = 0
	} else {
		var err error
		peerLastLogTerm, err = cm.log.GetTermAtIndex(peerLastLogIndex)
		if err != nil {
			return err
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
			return err
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
	return nil
}

// #RFS-L4: If there exists an N such that N > commitIndex, a majority
// of matchIndex[i] >= N, and log[N].term == currentTerm:
// set commitIndex = N (#5.3, #5.4)
func (cm *passiveConsensusModule) advanceCommitIndexIfPossible() error {
	newerCommitIndex, err := findNewerCommitIndex(
		cm.clusterInfo,
		cm.leaderVolatileState,
		cm.log,
		cm.persistentState.GetCurrentTerm(),
		cm.getCommitIndex(),
	)
	if err != nil {
		return err
	}
	if newerCommitIndex != 0 && newerCommitIndex > cm.getCommitIndex() {
		cm.setCommitIndex(newerCommitIndex)
	}
	return nil
}

// -- rpc bridging things

// This is an internal equivalent to RpcService without the reply setup.
type rpcSender interface {
	sendRpcAppendEntriesAsync(toServer ServerId, rpc *RpcAppendEntries)
	sendRpcRequestVoteAsync(toServer ServerId, rpc *RpcRequestVote)
}
