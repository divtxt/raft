package raft

import (
	"testing"
)

const (
	// Note: value for tests based on Figure 7
	TEST_CURRENT_TERM = 8
)

func setupTestFollower(logTerms []TermNo) *ConsensusModule {
	imle := makeIMLEWithDummyCommands(logTerms)
	ps := PersistentState{TEST_CURRENT_TERM}
	return NewConsensusModule(ps, imle)
}

func makeAEWithTerm(term TermNo) AppendEntries {
	return AppendEntries{term, 0, 0, nil, 0}
}

func makeAEWithTermAndPrevLogDetails(term TermNo, prevli LogIndex, prevterm TermNo) AppendEntries {
	return AppendEntries{term, prevli, prevterm, nil, 0}
}

// 1. Reply false if term < currentTerm (#5.1)
func TestRpcAELeaderTermLessThanCurrentTerm(t *testing.T) {
	follower := setupTestFollower(nil)
	followerTerm := follower.persistentState.currentTerm

	appendEntries := makeAEWithTerm(followerTerm - 1)

	reply, err := follower.processRpc(appendEntries)
	if err != nil {
		t.Fatal(err)
	}

	if reply.term != followerTerm {
		t.Error()
	}
	if reply.success != false {
		t.Error()
	}
}

// 2. Reply false if log doesn't contain an entry at prevLogIndex whose term
//      matches prevLogTerm (#5.3)
// Note: the above language is slightly ambiguous but I'm assuming that this
// step refers strictly to the follower's log not having any entry at
// prevLogIndex since step 3 covers the alternate conflicting entry case.
// Note: this test based on Figure 7, server (b)
func TestRpcAENoMatchingLogEntry(t *testing.T) {
	follower := setupTestFollower([]TermNo{1, 1, 1, 4})
	followerTerm := follower.persistentState.currentTerm

	appendEntries := makeAEWithTermAndPrevLogDetails(TEST_CURRENT_TERM, 10, 6)

	reply, err := follower.processRpc(appendEntries)
	if err != nil {
		t.Fatal(err)
	}

	if reply.term != followerTerm {
		t.Error()
	}
	if reply.success != false {
		t.Error()
	}

}

// 3. If an existing entry conflicts with a new one (same index
// but different terms), delete the existing entry and all that
// follow it (#5.3)
// 4. Append any new entries not already in the log
// 5. If leaderCommit > commitIndex, set commitIndex = min(leaderCommit,
// index of last new entry)
// Note: this is not specified, but I'm assuimg that the RPC reply should
// have success set to true.
// Note: this test case based on Figure 7, case (e) in the Raft paper but adds
// some extra entries to also test step 3
func TestRpcAEAppendNewEntries(t *testing.T) {
	follower := setupTestFollower([]TermNo{1, 1, 1, 4, 4, 4, 4, 4, 4, 4, 4})
	follower.volatileState.commitIndex = 3
	followerTerm := follower.persistentState.currentTerm

	if follower.log.getLogEntryAtIndex(6).Command != "c6" {
		t.Error()
	}

	sentLogEntries := []LogEntry{LogEntry{5, "c6'"}, LogEntry{5, "c7'"}, LogEntry{6, "c8'"}}

	appendEntries := AppendEntries{TEST_CURRENT_TERM, 5, 4, sentLogEntries, 7}

	reply, err := follower.processRpc(appendEntries)
	if err != nil {
		t.Fatal(err)
	}

	if reply.term != followerTerm {
		t.Error()
	}
	if !reply.success {
		t.Error(reply.success)
	}

	if iole := follower.log.getIndexOfLastEntry(); iole != 8 {
		t.Fatal(iole)
	}
	addedLogEntry := follower.log.getLogEntryAtIndex(6)
	if addedLogEntry.TermNo != 5 {
		t.Error()
	}
	if addedLogEntry.Command != "c6'" {
		t.Error()
	}

	// FIXME: need to test permutations in step 5
	if follower.volatileState.commitIndex != 7 {
		t.Error()
	}
}
