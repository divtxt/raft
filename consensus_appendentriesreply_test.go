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

// #RFS-A2: If RPC request or response contains term T > currentTerm:
// set currentTerm = T, convert to follower (#5.1)
// #5.1-p3s4: ...; if one server's current term is smaller than the
// other's, then it updates its current term to the larger value.
// #5.1-p3s5: If a candidate or leader discovers that its term is out of
// date, it immediately reverts to follower state.
func TestCM_RpcAER_Leader_NewerTerm(t *testing.T) {
	mcm, _ := testSetupMCM_Leader_Figure7LeaderLine(t)
	serverTerm := mcm.pcm.persistentState.GetCurrentTerm()
	sentRpc := makeAEWithTerm(serverTerm)

	mcm.pcm.rpcReply("s2", sentRpc, &RpcAppendEntriesReply{serverTerm + 1, false})
	if mcm.pcm.getServerState() != FOLLOWER {
		t.Fatal()
	}
	if mcm.pcm.persistentState.GetCurrentTerm() != serverTerm+1 {
		t.Fatal()
	}
	if mcm.pcm.persistentState.GetVotedFor() != "" {
		t.Fatal()
	}

}
