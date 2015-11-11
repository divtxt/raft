// AppendEntries RPC (Receiver Implementation)
// Invoked by leader to replicate log entries (#5.3); also used as heartbeat
// (#5.2).

package raft

import (
	"fmt"
)

func (cm *passiveConsensusModule) _processRpc_AppendEntries(appendEntries *RpcAppendEntries) bool {

	switch cm.serverState {
	case FOLLOWER:
		// Pass through to main logic below
	case CANDIDATE:
		panic("TODO: _processRpc_AppendEntries / CANDIDATE")
	case LEADER:
		panic("TODO: _processRpc_AppendEntries / LEADER")
	default:
		panic(fmt.Sprintf("FATAL: unknown ServerState: %v", cm.serverState))
	}

	leaderCurrentTerm := appendEntries.Term
	prevLogIndex := appendEntries.PrevLogIndex
	log := cm.log

	// 1. Reply false if term < currentTerm (#5.1)
	if leaderCurrentTerm < cm.persistentState.GetCurrentTerm() {
		return false
	}

	// 2. Reply false if log doesn't contain an entry at prevLogIndex whose
	// term matches prevLogTerm (#5.3)
	if log.getIndexOfLastEntry() < prevLogIndex {
		return false
	}

	// 3. If an existing entry conflicts with a new one (same index
	// but different terms), delete the existing entry and all that
	// follow it (#5.3)
	// 4. Append any new entries not already in the log
	log.setEntriesAfterIndex(prevLogIndex, appendEntries.Entries)

	// 5. If leaderCommit > commitIndex, set commitIndex = min(leaderCommit,
	// index of last new entry)
	leaderCommit := appendEntries.LeaderCommit
	if leaderCommit > cm.volatileState.commitIndex {
		indexOfLastNewEntry := log.getIndexOfLastEntry()
		if leaderCommit < indexOfLastNewEntry {
			cm.volatileState.commitIndex = leaderCommit
		} else {
			cm.volatileState.commitIndex = indexOfLastNewEntry
		}
	}

	return true
}
