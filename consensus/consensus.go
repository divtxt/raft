package consensus

import (
	"errors"
	"fmt"
	"log"
	"time"

	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/config"
	"github.com/divtxt/raft/consensus/candidate"
	"github.com/divtxt/raft/consensus/leader"
	"github.com/divtxt/raft/internal"
	"github.com/divtxt/raft/util"
)

type PassiveConsensusModule struct {
	// ===== the following fields meant to be immutable

	// -- External components
	RaftPersistentState         RaftPersistentState
	logRO                       internal.LogTailOnlyRO
	logWO                       internal.LogTailOnlyWO
	committer                   internal.ICommitter
	sendOnlyRpcRequestVoteAsync internal.SendOnlyRpcRequestVoteAsync
	aeSender                    internal.IAppendEntriesSender
	logger                      *log.Logger

	// -- Config
	ClusterInfo *config.ClusterInfo

	// ===== the following fields are mutable

	// -- State - for all servers
	serverState ServerState

	// commitIndex is the index of highest log entry known to be committed
	// (initialized to 0, increases monotonically)
	_commitIndex           LogIndex
	electionTimeoutChooser *util.ElectionTimeoutChooser
	ElectionTimeoutTimer   *util.Timer

	// -- State - for candidates only
	CandidateVolatileState *candidate.CandidateVolatileState

	// -- State - for leaders only
	LeaderVolatileState *leader.LeaderVolatileState
}

func NewPassiveConsensusModule(
	raftPersistentState RaftPersistentState,
	log internal.LogTailOnly,
	committer internal.ICommitter,
	sendOnlyRpcRequestVoteAsync internal.SendOnlyRpcRequestVoteAsync,
	aeSender internal.IAppendEntriesSender,
	clusterInfo *config.ClusterInfo,
	electionTimeoutLow time.Duration,
	nowFunc func() time.Time,
	logger *log.Logger,
) (*PassiveConsensusModule, error) {
	// Param checks
	if raftPersistentState == nil {
		return nil, errors.New("'raftPersistentState' cannot be nil")
	}
	if log == nil {
		return nil, errors.New("'log' cannot be nil")
	}
	if committer == nil {
		return nil, errors.New("'committer' cannot be nil")
	}
	if sendOnlyRpcRequestVoteAsync == nil {
		return nil, errors.New("'sendOnlyRpcRequestVoteAsync' cannot be nil")
	}
	if clusterInfo == nil {
		return nil, errors.New("clusterInfo cannot be nil")
	}
	if electionTimeoutLow.Nanoseconds() <= 0 {
		return nil, errors.New("electionTimeoutLow must be greater than zero")
	}

	electionTimeoutTimer := util.NewTimer(electionTimeoutLow, nowFunc)

	pcm := &PassiveConsensusModule{
		// -- External components
		raftPersistentState,
		log,
		log,
		committer,
		sendOnlyRpcRequestVoteAsync,
		aeSender,
		logger,

		// -- Config
		clusterInfo,

		// -- State - for all servers
		// #5.2-p1s2: When servers start up, they begin as followers
		FOLLOWER,

		// -- State - for all servers
		// commitIndex is the index of highest log entry known to be committed
		// (initialized to 0, increases monotonically)
		0,
		util.NewElectionTimeoutChooser(electionTimeoutLow),
		electionTimeoutTimer,

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
	return cm.serverState
}

// Set the current server state.
// Validates the server state before setting.
func (cm *PassiveConsensusModule) setServerState(serverState ServerState) {
	if serverState != FOLLOWER && serverState != CANDIDATE && serverState != LEADER {
		panic(fmt.Sprintf("FATAL: unknown ServerState: %v", serverState))
	}
	if cm.serverState != serverState {
		cm.logger.Println(
			"[raft] setServerState:",
			ServerStateToString(cm.serverState),
			"->",
			ServerStateToString(serverState),
		)
		cm.serverState = serverState
	}
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
	// FIXME: check against indexOfFirstEntry as well!
	iole, err := cm.logRO.GetIndexOfLastEntry()
	if err != nil {
		return err
	}
	if commitIndex > iole {
		return fmt.Errorf(
			"setCommitIndex to %v > current indexOfLastEntry %v",
			commitIndex,
			iole,
		)
	}
	cm._commitIndex = commitIndex
	err = cm.committer.CommitAsync(commitIndex)
	return err
}

// AppendCommand appends the given serialized command to the Raft log and applies it
// to the state machine once it is considered committed by the ConsensusModule.
func (cm *PassiveConsensusModule) AppendCommand(command Command) (<-chan CommandResult, error) {
	if cm.GetServerState() != LEADER {
		return nil, ErrNotLeader
	}

	termNo := cm.RaftPersistentState.GetCurrentTerm()
	logEntry := LogEntry{termNo, command}
	iole, err := cm.logWO.AppendEntry(logEntry)
	if err != nil {
		return nil, err
	}

	crc, err := cm.committer.RegisterListener(iole)
	if err != nil {
		return nil, err
	}

	return crc, nil
}

// Iterate
func (cm *PassiveConsensusModule) Tick() error {
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
		if cm.ElectionTimeoutTimer.Expired() {
			cm.logger.Println("[raft] Election timeout - starting a new election")
			err := cm.becomeCandidateAndBeginElection()
			if err != nil {
				return err
			}
			// *** SOLO ***
			// Single node cluster wins election immediately since it has all the votes
			// But don't skip the election process, mainly since it increases current term!
			if cm.ClusterInfo.GetClusterSize() == 1 {
				cm.logger.Println("[raft] Single node cluster - win election immediately!")
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

func (cm *PassiveConsensusModule) becomeCandidateAndBeginElection() error {
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
	cm.CandidateVolatileState, err = candidate.NewCandidateVolatileState(cm.ClusterInfo)
	if err != nil {
		return err
	}
	// FIXME: return newTerm to avoid logging here
	cm.logger.Println("[raft] becomeCandidateAndBeginElection: newTerm =", newTerm)
	cm.setServerState(CANDIDATE)
	// #5.2-p2s2: It then votes for itself and issues RequestVote RPCs
	// in parallel to each of the other servers in the cluster.
	err = cm.RaftPersistentState.SetVotedFor(cm.ClusterInfo.GetThisServerId())
	if err != nil {
		return err
	}
	lastLogIndex, lastLogTerm, err := GetIndexAndTermOfLastEntry(cm.logRO)
	if err != nil {
		return err
	}
	err = cm.ClusterInfo.ForEachPeer(
		func(serverId ServerId) error {
			rpcRequestVote := &RpcRequestVote{newTerm, lastLogIndex, lastLogTerm}
			cm.sendOnlyRpcRequestVoteAsync(serverId, rpcRequestVote)
			return nil
		},
	)
	if err != nil {
		return err
	}
	// Reset election timeout!
	cm.ElectionTimeoutTimer.RestartWithDuration(
		cm.electionTimeoutChooser.ChooseRandomElectionTimeout(),
	)
	return nil
}

func (cm *PassiveConsensusModule) becomeLeader() error {
	iole, err := cm.logRO.GetIndexOfLastEntry()
	if err != nil {
		return err
	}
	cm.LeaderVolatileState, err = leader.NewLeaderVolatileState(cm.ClusterInfo, iole, cm.aeSender)
	if err != nil {
		return err
	}
	cm.logger.Println("[raft] becomeLeader: iole =", iole, ", commitIndex =", cm._commitIndex)
	cm.setServerState(LEADER)
	// #RFS-L1a: Upon election: send initial empty AppendEntries RPCs (heartbeat)
	// to each server;
	err = cm.sendAppendEntriesToAllPeers(true)
	if err != nil {
		return err
	}
	return nil
}

func (cm *PassiveConsensusModule) becomeFollowerWithTerm(newTerm TermNo) error {
	currentTerm := cm.RaftPersistentState.GetCurrentTerm()
	if cm.GetServerState() == FOLLOWER && currentTerm == newTerm {
		// Nothing to change!
		return nil
	}
	cm.logger.Println("[raft] becomeFollowerWithTerm: newTerm =", newTerm)
	cm.setServerState(FOLLOWER)
	err := cm.RaftPersistentState.SetCurrentTerm(newTerm)
	if err != nil {
		return err
	}
	return nil
}

// -- leader code

func (cm *PassiveConsensusModule) sendAppendEntriesToAllPeers(empty bool) error {
	currentTerm := cm.RaftPersistentState.GetCurrentTerm()
	commitIndex := cm.GetCommitIndex()
	//
	return cm.ClusterInfo.ForEachPeer(
		func(serverId ServerId) error {
			fm, err := cm.LeaderVolatileState.GetFollowerManager(serverId)
			if err != nil {
				return err
			}
			return fm.SendAppendEntriesToPeerAsync(
				empty,
				currentTerm,
				commitIndex,
			)
		},
	)
}

// #RFS-L4: If there exists an N such that N > commitIndex, a majority
// of matchIndex[i] >= N, and log[N].term == currentTerm:
// set commitIndex = N (#5.3, #5.4)
func (cm *PassiveConsensusModule) advanceCommitIndexIfPossible() error {
	newerCommitIndex, err := cm.LeaderVolatileState.FindNewerCommitIndex(
		cm.ClusterInfo,
		cm.logRO,
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
		return fmt.Errorf(
			"FATAL: setEntriesAfterIndex(%d, ...) but commitIndex=%d", li, cm._commitIndex,
		)
	}
	newIole := li + LogIndex(len(entries))
	if newIole < cm._commitIndex {
		return fmt.Errorf(
			"FATAL: setEntriesAfterIndex(%d, ...) would set iole=%d < commitIndex=%d",
			li,
			newIole,
			cm._commitIndex,
		)
	}
	err := cm.committer.RemoveListenersAfterIndex(li)
	if err != nil {
		return nil
	}
	return cm.logWO.SetEntriesAfterIndex(li, entries)
}
