package raft

import (
	"testing"
)

func setupTestFollower() ConsensusModule {
	const TEST_FOLLOWER_TERM = 8
	return ConsensusModule{PersistentState{TEST_FOLLOWER_TERM}}
}

func makeAppendEntriesWithTerm(term TermNo) AppendEntries {
	return AppendEntries{term, 0, 0}
}

// 1. Reply false if term < currentTerm (#5.1)
func TestLeaderTermLessThanCurrentTerm(t *testing.T) {
	follower := setupTestFollower()
	followerTerm := follower.persistentState.currentTerm

	appendEntries := makeAppendEntriesWithTerm(followerTerm - 1)

	var reply AppendEntriesReply
	reply = follower.processRpc(appendEntries)

	if reply.term != followerTerm {
		t.Error()
	}

	if reply.success != false {
		t.Error()
	}
}
