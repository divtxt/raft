package raft

import (
	"testing"
)

const (
	// Note: value for tests based on Figure 7
	TEST_CURRENT_TERM = 8
)

func setupTestFollower(logTerms []TermNo) ConsensusModule {
	return ConsensusModule{PersistentState{TEST_CURRENT_TERM}}
}

func makeAEWithTerm(term TermNo) AppendEntries {
	return AppendEntries{term, 0, 0}
}

func makeAEWithTermAndPrevLogIndex(term TermNo, li LogIndex) AppendEntries {
	return AppendEntries{term, li, 0}
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

	appendEntries := makeAEWithTermAndPrevLogIndex(TEST_CURRENT_TERM, 10)

	var reply AppendEntriesReply
	reply = follower.processRpc(appendEntries)

	if reply.term != followerTerm {
		t.Error()
	}
	if reply.success != false {
		t.Error()
	}

}
