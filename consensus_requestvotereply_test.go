package raft

import (
	"testing"
)

// #5.2-p3s1: A candidate wins an election if it receives votes from a
// majority of the servers in the full cluster for the same term.
// #RFS-L1a: Upon election: send initial empty AppendEntries RPCs (heartbeat)
// to each server;
func TestCM_RpcRVR_Candidate_CandidateWinsElectionIfItReceivesMajorityOfVotes(t *testing.T) {
	mcm, mrs := testSetupMCM_Candidate_Figure7LeaderLine(t)
	serverTerm := mcm.pcm.persistentState.GetCurrentTerm()

	// s2 grants vote - should stay as candidate
	mcm.pcm.rpcReply("s2", &RpcRequestVoteReply{serverTerm, true})
	if mcm.pcm.getServerState() != CANDIDATE {
		t.Fatal()
	}

	// s3 denies vote - should stay as candidate
	mcm.pcm.rpcReply("s3", &RpcRequestVoteReply{serverTerm, false})
	if mcm.pcm.getServerState() != CANDIDATE {
		t.Fatal()
	}

	// s4 grants vote - should become leader
	mcm.pcm.rpcReply("s4", &RpcRequestVoteReply{serverTerm, true})
	if mcm.pcm.getServerState() != LEADER {
		t.Fatal()
	}

	// leader setup
	lastLogIndex, lastLogTerm := getIndexAndTermOfLastEntry(mcm.pcm.log)
	expectedRpc := &RpcAppendEntries{
		serverTerm,
		lastLogIndex,
		lastLogTerm,
		[]LogEntry{},
		0, // TODO: tests for this?!
	}
	expectedRpcs := []mockSentRpc{
		{"s2", expectedRpc},
		{"s3", expectedRpc},
		{"s4", expectedRpc},
		{"s5", expectedRpc},
	}
	mrs.checkSentRpcs(t, expectedRpcs)

	// s5 grants vote - should stay leader
	mcm.pcm.rpcReply("s5", &RpcRequestVoteReply{serverTerm, true})
	if mcm.pcm.getServerState() != LEADER {
		t.Fatal()
	}
}

// #5.2-p5s1: The third possible outcome is that a candidate neither
// wins nor loses the election; ... votes could be split so that no
// candidate obtains a majority.
// #5.2-p5s2: When this happens, each candidate will time out and
// start a new election by incrementing its term and initiating
// another round of RequestVote RPCs.
func TestCM_RpcRVR_Candidate_StartNewElectionOnElectionTimeout(t *testing.T) {
	mcm, mrs := testSetupMCM_Candidate_Figure7LeaderLine(t)
	serverTerm := mcm.pcm.persistentState.GetCurrentTerm()

	// s2 grants vote - should stay as candidate
	mcm.pcm.rpcReply("s2", &RpcRequestVoteReply{serverTerm, true})
	if mcm.pcm.getServerState() != CANDIDATE {
		t.Fatal()
	}

	// s3 denies vote - should stay as candidate
	mcm.pcm.rpcReply("s3", &RpcRequestVoteReply{serverTerm, false})
	if mcm.pcm.getServerState() != CANDIDATE {
		t.Fatal()
	}

	// no more votes - election timeout causes a new election
	testCM_FollowerOrCandidate_StartsElectionOnElectionTimeout_Part2(t, mcm, mrs, testCurrentTerm+2)
}

// Extra: follower or leader ignores vote
func TestCM_RpcRVR_FollowerOrLeader_Ignores(t *testing.T) {
	f := func(setup func(t *testing.T) (mcm *managedConsensusModule, mrs *mockRpcSender)) {
		mcm, mrs := setup(t)
		serverTerm := mcm.pcm.persistentState.GetCurrentTerm()
		beforeState := mcm.pcm.getServerState()

		// s2 grants vote - ignore
		mcm.pcm.rpcReply("s2", &RpcRequestVoteReply{serverTerm, true})
		if mcm.pcm.getServerState() != beforeState {
			t.Fatal()
		}
		mrs.checkSentRpcs(t, []mockSentRpc{})

		// s3 denies vote - ignore
		mcm.pcm.rpcReply("s3", &RpcRequestVoteReply{serverTerm, false})
		if mcm.pcm.getServerState() != beforeState {
			t.Fatal()
		}
		mrs.checkSentRpcs(t, []mockSentRpc{})
	}

	f(testSetupMCM_Leader_Figure7LeaderLine)
}
