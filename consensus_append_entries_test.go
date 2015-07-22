package raft

import (
	"testing"
)

const (
	// Note: value for tests based on Figure 7
	TEST_CURRENT_TERM = 8
)

func setupTestFollower(logTerms []TermNo) ConsensusModule {
	imle := makeIMLEWithDummyCommands(logTerms)
	return ConsensusModule{PersistentState{TEST_CURRENT_TERM, imle}}
}

func makeAEWithTerm(term TermNo) AppendEntries {
	return AppendEntries{term, 0, 0, nil}
}

func makeAEWithTermAndPrevLogDetails(term TermNo, prevli LogIndex, prevterm TermNo) AppendEntries {
	return AppendEntries{term, prevli, prevterm, nil}
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
// Note: this test case based on Figure 7, case (f) in the Raft paper
func TestRpcAEConflictingLogEntry(t *testing.T) {
	follower := setupTestFollower([]TermNo{1, 1, 1, 2, 2, 2, 3, 3, 3, 3, 3})
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

	if iole := follower.persistentState.log.getIndexOfLastEntry(); iole != 9 {
		t.Error(iole)
	}
}

// 4. Append any new entries not already in the log
// Note: this is not specified, but I'm assuimg that the RPC reply should
// have success set to true.
// Note: this test case based on Figure 7, case (a) in the Raft paper
func TestRpcAEAppendNewEntries(t *testing.T) {
	follower := setupTestFollower([]TermNo{1, 1, 1, 4, 4, 5, 5, 6, 6})
	followerTerm := follower.persistentState.currentTerm

	sentLogEntries := []LogEntry{LogEntry{6, "c10"}}

	appendEntries := AppendEntries{TEST_CURRENT_TERM, 9, 6, sentLogEntries}

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

	if iole := follower.persistentState.log.getIndexOfLastEntry(); iole != 10 {
		t.Fatal(iole)
	}
	addedLogEntry := follower.persistentState.log.getLogEntryAtIndex(10)
	if addedLogEntry.TermNo != 6 {
		t.Error()
	}
	if addedLogEntry.Command != "c10" {
		t.Error()
	}
}
