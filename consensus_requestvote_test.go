package raft

import (
	"reflect"
	"testing"
)

// 1. Reply false if term < currentTerm (#5.1)
// Note: test based on Figure 7; server is leader line; peer is case (a)
func TestCM_RpcRV_TermLessThanCurrentTerm(t *testing.T) {
	f := func(setup func(t *testing.T) (mcm *managedConsensusModule, mrs *mockRpcSender)) {
		mcm, _ := setup(t)
		serverTerm := mcm.pcm.persistentState.GetCurrentTerm()

		requestVote := &RpcRequestVote{7, 9, 6}

		reply := mcm.pcm.rpc("s2", requestVote)

		expectedRpc := &RpcRequestVoteReply{serverTerm, false}
		if !reflect.DeepEqual(reply, expectedRpc) {
			t.Fatal(reply)
		}
	}

	f(testSetupMCM_Follower_Figure7LeaderLine)
	f(testSetupMCM_Candidate_Figure7LeaderLine)
	f(testSetupMCM_Leader_Figure7LeaderLine)
}
