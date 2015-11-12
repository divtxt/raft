package raft

import (
	"testing"
)

func makeRVWithTerm(term TermNo) *RpcRequestVote {
	return &RpcRequestVote{term, 0, 0}
}

// 1. Reply false if term < currentTerm (#5.1)
// Note: this test assumes follower in sync with the Figure 7 leader
func TestCM_RpcRVR_Follower_TermLessThanCurrentTerm_Leader(t *testing.T) {
	terms := makeLogTerms_Figure7LeaderLine()
	mcm, mrs := setupManagedConsensusModuleR2(t, terms)
	followerTerm := mcm.pcm.persistentState.GetCurrentTerm()

	if mcm.pcm.getServerState() != FOLLOWER {
		t.Fatal()
	}

	requestVote := makeRVWithTerm(followerTerm - 1)

	mcm.pcm.rpc("s2", requestVote)

	expectedRpc := &RpcRequestVoteReply{false}
	expectedRpcs := []mockSentRpc{
		{"s2", expectedRpc},
	}
	mrs.checkSentRpcs(t, expectedRpcs)
}

// TODO: test step 1 for other states
