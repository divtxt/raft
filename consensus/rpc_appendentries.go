// AppendEntries RPC (Receiver Implementation)
// Invoked by leader to replicate log entries (#5.3); also used as heartbeat
// (#5.2).

package consensus

import (
	"fmt"
	"time"

	. "github.com/divtxt/raft"
)

// Process the given RpcAppendEntries message
// #RFS-F1: Respond to RPCs from candidates and leaders
func (cm *PassiveConsensusModule) Rpc_RpcAppendEntries(
	from ServerId,
	appendEntries *RpcAppendEntries,
	now time.Time,
) (*RpcAppendEntriesReply, error) {
	if from == cm.ClusterInfo.GetThisServerId() {
		return nil, fmt.Errorf("FATAL: from server has same serverId: %v", cm.ClusterInfo.GetThisServerId())
	}
	if !cm.ClusterInfo.IsPeer(from) {
		return nil, fmt.Errorf("FATAL: 'from' serverId %v is not in the cluster", from)
	}

	makeReply := func(success bool) *RpcAppendEntriesReply {
		return &RpcAppendEntriesReply{
			cm.RaftPersistentState.GetCurrentTerm(), // refetch in case it has changed!
			success,
		}
	}

	serverTerm := cm.RaftPersistentState.GetCurrentTerm()
	leaderCurrentTerm := appendEntries.Term
	prevLogIndex := appendEntries.PrevLogIndex

	// 1. Reply false if term < currentTerm (#5.1)
	if leaderCurrentTerm < serverTerm {
		return makeReply(false), nil
	}

	// Extra: raft violation - two leaders with same term
	if cm.GetServerState() == LEADER && leaderCurrentTerm == serverTerm {
		return nil, fmt.Errorf(
			"FATAL: two leaders with same term - got AppendEntries from: %v with term: %v",
			from,
			serverTerm,
		)
	}

	// #RFS-F2: (paraphrasing) AppendEntries RPC from current leader should
	// prevent election timeout
	cm.ElectionTimeoutTracker.Touch(now)

	// #RFS-A2: If RPC request or response contains term T > currentTerm:
	// set currentTerm = T, convert to follower (#5.1)
	// #RFS-C3: If AppendEntries RPC received from new leader:
	// convert to follower
	// #5.1-p3s4: ...; if one server's current term is smaller than the
	// other's, then it updates its current term to the larger value.
	// #5.1-p3s5: If a candidate or leader discovers that its term is out of
	// date, it immediately reverts to follower state.
	// #5.2-p4s1: While waiting for votes, a candidate may receive an
	// AppendEntries RPC from another server claiming to be leader.
	// #5.2-p4s2: If the leader’s term (included in its RPC) is at least as
	// large as the candidate’s current term, then the candidate recognizes
	// the leader as legitimate and returns to follower state.
	err := cm.becomeFollowerWithTerm(leaderCurrentTerm)
	if err != nil {
		return nil, err
	}

	// 2. Reply false if log doesn't contain an entry at prevLogIndex whose
	// term matches prevLogTerm (#5.3)
	iole, err := cm.LogRO.GetIndexOfLastEntry()
	if err != nil {
		return nil, err
	}
	if iole < prevLogIndex {
		return makeReply(false), nil
	}

	// 3. If an existing entry conflicts with a new one (same index
	// but different terms), delete the existing entry and all that
	// follow it (#5.3)
	// 4. Append any new entries not already in the log
	err = cm.setEntriesAfterIndex(prevLogIndex, appendEntries.Entries)
	if err != nil {
		return nil, err
	}

	// 5. If leaderCommit > commitIndex, set commitIndex = min(leaderCommit,
	// index of last new entry)
	leaderCommit := appendEntries.LeaderCommit
	if leaderCommit > cm.GetCommitIndex() {
		var indexOfLastNewEntry LogIndex
		indexOfLastNewEntry, err = cm.LogRO.GetIndexOfLastEntry()
		if err != nil {
			return nil, err
		}
		if leaderCommit < indexOfLastNewEntry {
			err = cm.setCommitIndex(leaderCommit)
			if err != nil {
				return nil, err
			}
		} else {
			err = cm.setCommitIndex(indexOfLastNewEntry)
			if err != nil {
				return nil, err
			}
		}
	}

	return makeReply(true), nil
}
