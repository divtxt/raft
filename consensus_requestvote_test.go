package raft

import (
	"reflect"
	"testing"
)

func makeRVWithTerm(term TermNo) *RpcRequestVote {
	return &RpcRequestVote{term, 0, 0}
}

// 1. Reply false if term < currentTerm (#5.1)
// Note: this test assumes server in sync with the Figure 7 leader
func TestCM_RpcRV_TermLessThanCurrentTerm(t *testing.T) {
	f := func(setup func(t *testing.T) (mcm *managedConsensusModule, mrs *mockRpcSender)) {
		mcm, _ := setup(t)
		serverTerm := mcm.pcm.persistentState.GetCurrentTerm()

		requestVote := makeRVWithTerm(serverTerm - 1)

		reply := mcm.pcm.rpc("s2", requestVote)

		expectedRpc := &RpcRequestVoteReply{false}
		if !reflect.DeepEqual(reply, expectedRpc) {
			t.Fatal(reply)
		}
	}

	f(testSetupMCM_Follower_Figure7LeaderLine)
	f(testSetupMCM_Candidate_Figure7LeaderLine)
	f(testSetupMCM_Leader_Figure7LeaderLine)
}
