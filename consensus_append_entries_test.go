package raft

import (
	"testing"
)

const (
	// Note: value for tests based on Figure 7
	TEST_CURRENT_TERM = 8
)

func setupTestFollower(logTerms []TermNo) ConsensusModule {
	imle := new(inMemoryLog)
	imle.terms = []TermNo{1, 1, 1, 4, 4, 5, 5, 6, 6, 6}
	return ConsensusModule{PersistentState{TEST_CURRENT_TERM, imle}}
}

func makeAEWithTerm(term TermNo) AppendEntries {
	return AppendEntries{term, 0, 0}
}

func makeAEWithTermAndPrevLogDetails(term TermNo, prevli LogIndex, prevterm TermNo) AppendEntries {
	return AppendEntries{term, prevli, prevterm}
}

// 1. Reply false if term < currentTerm (#5.1)
func TestRpcAELeaderTermLessThanCurrentTerm(t *testing.T) {
	follower := setupTestFollower(nil)
	followerTerm := follower.persistentState.currentTerm

	appendEntries := makeAEWithTerm(followerTerm - 1)

	var reply AppendEntriesReply
	reply = follower.processRpc(appendEntries)

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

	var reply AppendEntriesReply
	reply = follower.processRpc(appendEntries)

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

	var reply AppendEntriesReply
	reply = follower.processRpc(appendEntries)

	if reply.term != followerTerm {
		t.Error()
	}
	if reply.success != false {
		t.Error()
	}

	if follower.persistentState.log.getIndexOfLastEntry() != 9 {
		t.Error()
	}
}
