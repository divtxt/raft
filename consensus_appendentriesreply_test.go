package raft

import (
	"testing"
)

// Extra: follower ignores
func TestCM_RpcAER_Follower_Ignores(t *testing.T) {
	mcm, mrs := testSetupMCM_Follower_Figure7LeaderLine(t)
	serverTerm := mcm.pcm.persistentState.GetCurrentTerm()

	mcm.pcm.rpcReply("s2", &RpcAppendEntriesReply{serverTerm, true})
	if mcm.pcm.getServerState() != FOLLOWER {
		t.Fatal()
	}
	mrs.checkSentRpcs(t, []mockSentRpc{})
}

// Extra: candidate ignores
func TestCM_RpcAER_Candidate_Ignores(t *testing.T) {
	mcm, mrs := testSetupMCM_Candidate_Figure7LeaderLine(t)
	serverTerm := mcm.pcm.persistentState.GetCurrentTerm()

	mcm.pcm.rpcReply("s2", &RpcAppendEntriesReply{serverTerm, true})
	if mcm.pcm.getServerState() != CANDIDATE {
		t.Fatal()
	}
	mrs.checkSentRpcs(t, []mockSentRpc{})
}
