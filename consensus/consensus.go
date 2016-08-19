package consensus

import (
	"errors"
	"fmt"
	. "github.com/divtxt/raft"
	config "github.com/divtxt/raft/config"
	consensus_state "github.com/divtxt/raft/consensus/state"
	util "github.com/divtxt/raft/util"
	"sync/atomic"
	"time"
)

type PassiveConsensusModule struct {
	// ===== the following fields meant to be immutable

	// -- External components
	RaftPersistentState RaftPersistentState
	LogRO               LogReadOnly
	_log                Log
	_stateMachine       StateMachine
	RpcSendOnly         RpcSendOnly

	// -- Config
	ClusterInfo *config.ClusterInfo

	// ===== the following fields may be accessed concurrently

	// -- State - for all servers
	_unsafe_serverState ServerState

	// ===== the following fields meant for single-threaded access

	// -- State - for all servers
	// commitIndex is the index of highest log entry known to be committed
	// (initialized to 0, increases monotonically)
	_commitIndex           LogIndex
	ElectionTimeoutTracker *util.ElectionTimeoutTracker

	// -- State - for candidates only
	CandidateVolatileState *consensus_state.CandidateVolatileState

	// -- State - for leaders only
	LeaderVolatileState *consensus_state.LeaderVolatileState
}

func NewPassiveConsensusModule(
	raftPersistentState RaftPersistentState,
	log Log,
	stateMachine StateMachine,
	rpcSendOnly RpcSendOnly,
	clusterInfo *config.ClusterInfo,
	electionTimeoutLow time.Duration,
	now time.Time,
) (*PassiveConsensusModule, error) {
	// Param checks
	if raftPersistentState == nil {
		return nil, errors.New("'raftPersistentState' cannot be nil")
	}
	if log == nil {
		return nil, errors.New("'log' cannot be nil")
	}
	if stateMachine == nil {
		return nil, errors.New("'stateMachine' cannot be nil")
	}
	if rpcSendOnly == nil {
		return nil, errors.New("'rpcSendOnly' cannot be nil")
	}
	if clusterInfo == nil {
		return nil, errors.New("clusterInfo cannot be nil")
	}
	if electionTimeoutLow.Nanoseconds() <= 0 {
		return nil, errors.New("electionTimeoutLow must be greater than zero")
	}

	pcm := &PassiveConsensusModule{
		// -- External components
		raftPersistentState,
		log,
		log,
		stateMachine,
		rpcSendOnly,

		// -- Config
		clusterInfo,

		// -- State - for all servers
		// #5.2-p1s2: When servers start up, they begin as followers
		FOLLOWER,

		// -- State - for all servers
		// commitIndex is the index of highest log entry known to be committed
		// (initialized to 0, increases monotonically)
		0,
		util.NewElectionTimeoutTracker(electionTimeoutLow, now),

		// -- State - for candidates only
		nil,

		// -- State - for leaders only
		nil,
	}

	return pcm, nil
}

// Get the current server state.
// Validates the server state before returning.
func (cm *PassiveConsensusModule) GetServerState() ServerState {
	return ServerState(atomic.LoadUint32((*uint32)(&cm._unsafe_serverState)))
}

// Set the current server state.
// Validates the server state before setting.
func (cm *PassiveConsensusModule) setServerState(serverState ServerState) error {
	if serverState != FOLLOWER && serverState != CANDIDATE && serverState != LEADER {
		return fmt.Errorf("FATAL: unknown ServerState: %v", serverState)
	}
	atomic.StoreUint32((*uint32)(&cm._unsafe_serverState), (uint32)(serverState))
	return nil
}

// Get the current commitIndex value.
func (cm *PassiveConsensusModule) GetCommitIndex() LogIndex {
	return cm._commitIndex
}

// Set the current commitIndex value.
// Checks that it is does not reduce.
func (cm *PassiveConsensusModule) setCommitIndex(commitIndex LogIndex) error {
	if commitIndex < cm._commitIndex {
		return fmt.Errorf(
			"setCommitIndex to %v < current commitIndex %v",
			commitIndex,
			cm._commitIndex,
		)
	}
	cm._commitIndex = commitIndex
	err := cm._stateMachine.CommitIndexChanged(commitIndex)
	if err != nil {
		return err
	}
	return nil
}

// Append the given command as an entry in the log.
// #RFS-L2a: If command received from client: append entry to local log
//
// The unserialized command will be sent to StateMachine.ReviewAppendCommand() for review
// and serialization. If StateMachine.ReviewAppendCommand() approves the command, the serialized
// command is appended to the log using Log.AppendEntry().
//
// Returns the reply from StateMachine.ReviewAppendCommand() or nil if not currently the leader.
func (cm *PassiveConsensusModule) AppendCommand(
	rawCommand interface{},
) (interface{}, error) {
	if cm.GetServerState() != LEADER {
		return nil, nil
	}

	_, result, err := cm.appendEntry(cm.RaftPersistentState.GetCurrentTerm(), rawCommand)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// Iterate
func (cm *PassiveConsensusModule) Tick(now time.Time) error {
	serverState := cm.GetServerState()
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
		if cm.ElectionTimeoutTracker.ElectionTimeoutHasOccurred(now) {
			err := cm.becomeCandidateAndBeginElection(now)
			if err != nil {
				return err
			}
			// *** SOLO ***
			// Single node cluster wins election immediately since it has all the votes
			// But don't skip the election process, mainly since it increases current term!
			if cm.ClusterInfo.GetClusterSize() == 1 {
				err := cm.becomeLeader()
				if err != nil {
					return err
				}
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

func (cm *PassiveConsensusModule) becomeCandidateAndBeginElection(now time.Time) error {
	// #RFS-C1: On conversion to candidate, start election:
	// Increment currentTerm; Vote for self; Send RequestVote RPCs
	// to all other servers; Reset election timer
	// #5.2-p2s1: To begin an election, a follower increments its
	// current term and transitions to candidate state.
	newTerm := cm.RaftPersistentState.GetCurrentTerm() + 1
	err := cm.RaftPersistentState.SetCurrentTerm(newTerm)
	if err != nil {
		return err
	}
	cm.CandidateVolatileState, err = consensus_state.NewCandidateVolatileState(cm.ClusterInfo)
	if err != nil {
		return err
	}
	err = cm.setServerState(CANDIDATE)
	if err != nil {
		return err
	}
	// #5.2-p2s2: It then votes for itself and issues RequestVote RPCs
	// in parallel to each of the other servers in the cluster.
	err = cm.RaftPersistentState.SetVotedFor(cm.ClusterInfo.GetThisServerId())
	if err != nil {
		return err
	}
	lastLogIndex, lastLogTerm, err := GetIndexAndTermOfLastEntry(cm.LogRO)
	if err != nil {
		return err
	}
	err = cm.ClusterInfo.ForEachPeer(
		func(serverId ServerId) error {
			rpcRequestVote := &RpcRequestVote{newTerm, lastLogIndex, lastLogTerm}
			return cm.RpcSendOnly.SendOnlyRpcRequestVoteAsync(serverId, rpcRequestVote)
		},
	)
	if err != nil {
		return err
	}
	// Reset election timeout!
	cm.ElectionTimeoutTracker.ChooseNewRandomElectionTimeoutAndTouch(now)
	return nil
}

func (cm *PassiveConsensusModule) becomeLeader() error {
	iole, err := cm.LogRO.GetIndexOfLastEntry()
	if err != nil {
		return err
	}
	cm.LeaderVolatileState, err = consensus_state.NewLeaderVolatileState(cm.ClusterInfo, iole)
	if err != nil {
		return err
	}
	err = cm.setServerState(LEADER)
	if err != nil {
		return err
	}
	// #RFS-L1a: Upon election: send initial empty AppendEntries RPCs (heartbeat)
	// to each server;
	err = cm.sendAppendEntriesToAllPeers(true)
	if err != nil {
		return err
	}
	return nil
}

func (cm *PassiveConsensusModule) becomeFollowerWithTerm(newTerm TermNo) error {
	err := cm.setServerState(FOLLOWER)
	if err != nil {
		return err
	}
	err = cm.RaftPersistentState.SetCurrentTerm(newTerm)
	if err != nil {
		return err
	}
	return nil
}

// -- leader code

func (cm *PassiveConsensusModule) sendAppendEntriesToAllPeers(empty bool) error {
	return cm.ClusterInfo.ForEachPeer(
		func(serverId ServerId) error {
			return cm.sendAppendEntriesToPeer(serverId, empty)
		},
	)
}

func (cm *PassiveConsensusModule) sendAppendEntriesToPeer(
	peerId ServerId,
	empty bool,
) error {
	serverTerm := cm.RaftPersistentState.GetCurrentTerm()
	//
	peerNextIndex, err := cm.LeaderVolatileState.GetNextIndex(peerId)
	if err != nil {
		return err
	}
	peerLastLogIndex := peerNextIndex - 1
	var peerLastLogTerm TermNo
	if peerLastLogIndex == 0 {
		peerLastLogTerm = 0
	} else {
		var err error
		peerLastLogTerm, err = cm.LogRO.GetTermAtIndex(peerLastLogIndex)
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
		entriesToSend, err = cm.LogRO.GetEntriesAfterIndex(peerLastLogIndex)
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
		cm.GetCommitIndex(),
	}
	return cm.RpcSendOnly.SendOnlyRpcAppendEntriesAsync(peerId, rpcAppendEntries)
}

// #RFS-L4: If there exists an N such that N > commitIndex, a majority
// of matchIndex[i] >= N, and log[N].term == currentTerm:
// set commitIndex = N (#5.3, #5.4)
func (cm *PassiveConsensusModule) advanceCommitIndexIfPossible() error {
	newerCommitIndex, err := consensus_state.FindNewerCommitIndex(
		cm.ClusterInfo,
		cm.LeaderVolatileState,
		cm.LogRO,
		cm.RaftPersistentState.GetCurrentTerm(),
		cm.GetCommitIndex(),
	)
	if err != nil {
		return err
	}
	if newerCommitIndex != 0 && newerCommitIndex > cm.GetCommitIndex() {
		err = cm.setCommitIndex(newerCommitIndex)
		if err != nil {
			return err
		}
	}
	return nil
}

// Wrapper for the call to Log.SetEntriesAfterIndex()
func (cm *PassiveConsensusModule) setEntriesAfterIndex(li LogIndex, entries []LogEntry) error {
	// Check that we're not trying to rewind past commitIndex
	if li < cm._commitIndex {
		return fmt.Errorf("FATAL: setEntriesAfterIndex(%d, ...) but commitIndex=%d", li, cm._commitIndex)
	}
	return cm._log.SetEntriesAfterIndex(li, entries)
}

// Append a new entry with the given term and given raw command.
//
// Wrapper around StateMachine.ReviewAppendCommand() and Log.AppendEntry().
//
// This method should only be called when this ConsensusModule is the leader.
//
// Should return (<appended>, <reply>, nil) where <appended> is a boolean indicating whether or
// not the command was accepted and appended to the log, and <reply> is an appropriate
// unserialized reply.
func (cm *PassiveConsensusModule) appendEntry(
	termNo TermNo,
	rawCommand interface{},
) (bool, interface{}, error) {
	// Get the command checked and serialized
	command, reply, err := cm._stateMachine.ReviewAppendCommand(rawCommand)
	if err != nil {
		return false, nil, err
	}

	// If command rejected, return without appending to the log immediately.
	if command == nil {
		return false, reply, nil
	}

	// Append serialized command to the log.
	err = cm._log.AppendEntry(termNo, command)
	if err != nil {
		return false, nil, err
	}

	return true, reply, nil
}

// -- rpc bridging things

// This is an internal equivalent to RpcService without the reply part.
type RpcSendOnly interface {
	SendOnlyRpcAppendEntriesAsync(toServer ServerId, rpc *RpcAppendEntries) error
	SendOnlyRpcRequestVoteAsync(toServer ServerId, rpc *RpcRequestVote) error
}
