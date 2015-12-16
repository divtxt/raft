package raft

import (
	"fmt"
	"testing"
)

// Extra: ignore replies for previous term rpc
func TestCM_RpcAER_All_IgnorePreviousTermRpc(t *testing.T) {
	f := func(setup func(t *testing.T) (mcm *managedConsensusModule, mrs *mockRpcSender)) {
		mcm, mrs := setup(t)
		serverTerm := mcm.pcm.persistentState.GetCurrentTerm()
		sentRpc := makeAEWithTerm(serverTerm - 1)
		beforeState := mcm.pcm.getServerState()

		mcm.pcm.rpcReply("s2", sentRpc, &RpcAppendEntriesReply{serverTerm, true})
		if mcm.pcm.getServerState() != beforeState {
			t.Fatal()
		}
		mrs.checkSentRpcs(t, []mockSentRpc{})
	}

	f(testSetupMCM_Follower_Figure7LeaderLine)
	f(testSetupMCM_Candidate_Figure7LeaderLine)
	f(testSetupMCM_Leader_Figure7LeaderLine)
}

// Extra: raft violation - only leader can get AppendEntriesReply
func TestCM_RpcAER_FollowerOrCandidate_PanicsForSameTermReply(t *testing.T) {
	f := func(setup func(t *testing.T) (mcm *managedConsensusModule, mrs *mockRpcSender)) {
		mcm, _ := setup(t)
		serverTerm := mcm.pcm.persistentState.GetCurrentTerm()
		sentRpc := makeAEWithTerm(serverTerm)

		test_ExpectPanic(
			t,
			func() {
				mcm.pcm.rpcReply("s2", sentRpc, &RpcAppendEntriesReply{serverTerm, true})
			},
			fmt.Sprintf("FATAL: non-leader got AppendEntriesReply from: s2 with term: %v", serverTerm),
		)
	}

	f(testSetupMCM_Follower_Figure7LeaderLine)
	f(testSetupMCM_Candidate_Figure7LeaderLine)
}
