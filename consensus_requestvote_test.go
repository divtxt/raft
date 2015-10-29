package raft

import (
	"reflect"
	"testing"
)

func makeRVWithTerm(term TermNo) *RpcRequestVote {
	return &RpcRequestVote{term, 0, 0}
}

// 1. Reply false if term < currentTerm (#5.1)
// Note: this test assumes follower in sync with the Figure 7 leader
func TestRpcRVR_Follower_TermLessThanCurrentTerm_Leader(t *testing.T) {
	terms := testLogTerms_Figure7LeaderLine()
	mcm, mrs := setupManagedConsensusModuleR2(t, terms)
	followerTerm := mcm.pcm.persistentState.GetCurrentTerm()

	if mcm.pcm.getServerState() != FOLLOWER {
		t.Fatal()
	}

	requestVote := makeRVWithTerm(followerTerm - 1)

	mcm.pcm.rpc("s2", requestVote)

	sentRpcs := mrs.getAllSortedByToServer()
	if len(sentRpcs) != 1 {
		t.Error(len(sentRpcs))
	}
	sentRpc := sentRpcs[0]
	if sentRpc.toServer != "s2" {
		t.Error()
	}
	expectedRpc := &RpcRequestVoteReply{false}
	if !reflect.DeepEqual(sentRpc.rpc, expectedRpc) {
		t.Fatal(sentRpc.rpc, expectedRpc)
	}
}

// TODO: test step 1 for other states
