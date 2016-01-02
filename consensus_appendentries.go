// AppendEntries RPC (Receiver Implementation)
// Invoked by leader to replicate log entries (#5.3); also used as heartbeat
// (#5.2).

package raft

import (
	"fmt"
	"time"
)

func (cm *passiveConsensusModule) _processRpc_AppendEntries(
	serverState ServerState,
	from ServerId,
	appendEntries *RpcAppendEntries,
	now time.Time,
) bool {

	switch serverState {
	case FOLLOWER:
		// Pass through to main logic below
	case CANDIDATE:
		// Pass through to main logic below
	case LEADER:
		// Pass through to main logic below
	}

	serverTerm := cm.persistentState.GetCurrentTerm()
	leaderCurrentTerm := appendEntries.Term
	prevLogIndex := appendEntries.PrevLogIndex
	log := cm.log

	// 1. Reply false if term < currentTerm (#5.1)
	if leaderCurrentTerm < serverTerm {
		return false
	}

	// Extra: raft violation - two leaders with same term
	if serverState == LEADER && leaderCurrentTerm == serverTerm {
		panic(fmt.Sprintf(
			"FATAL: two leaders with same term - got AppendEntries from: %v with term: %v",
			from,
			serverTerm,
		))
	}

	// #RFS-F2: (paraphrasing) AppendEntries RPC from current leader should
	// prevent election timeout
	cm.electionTimeoutTracker.touch(now)

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
	cm.becomeFollowerWithTerm(leaderCurrentTerm)

	// 2. Reply false if log doesn't contain an entry at prevLogIndex whose
	// term matches prevLogTerm (#5.3)
	if log.GetIndexOfLastEntry() < prevLogIndex {
		return false
	}

	// 3. If an existing entry conflicts with a new one (same index
	// but different terms), delete the existing entry and all that
	// follow it (#5.3)
	// 4. Append any new entries not already in the log
	log.SetEntriesAfterIndex(prevLogIndex, appendEntries.Entries)

	// 5. If leaderCommit > commitIndex, set commitIndex = min(leaderCommit,
	// index of last new entry)
	leaderCommit := appendEntries.LeaderCommit
	if leaderCommit > cm.getCommitIndex() {
		indexOfLastNewEntry := log.GetIndexOfLastEntry()
		if leaderCommit < indexOfLastNewEntry {
			cm.setCommitIndex(leaderCommit)
		} else {
			cm.setCommitIndex(indexOfLastNewEntry)
		}
	}

	return true
}
