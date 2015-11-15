package raft

import (
	"testing"
)

// #5.2-p3s1: A candidate wins an election if it receives votes from a
// majority of the servers in the full cluster for the same term.
func TestCM_RpcRVR_Candidate_CandidateWinsElectionIfItReceivesMajorityOfVotes(t *testing.T) {
	mcm, _ := testSetupMCM_Candidate_Figure7LeaderLine(t)

	// s2 grants vote - should stay as candidate
	mcm.pcm.rpc("s2", &RpcRequestVoteReply{true})
	if mcm.pcm.getServerState() != CANDIDATE {
		t.Fatal()
	}

	// s3 denies vote - should stay as candidate
	mcm.pcm.rpc("s3", &RpcRequestVoteReply{false})
	if mcm.pcm.getServerState() != CANDIDATE {
		t.Fatal()
	}

	// s4 grants vote - should become leader
	mcm.pcm.rpc("s4", &RpcRequestVoteReply{true})
	if mcm.pcm.getServerState() != LEADER {
		t.Fatal()
	}

	// s5 grants vote - should stay leader
	mcm.pcm.rpc("s5", &RpcRequestVoteReply{true})
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

	// s2 grants vote - should stay as candidate
	mcm.pcm.rpc("s2", &RpcRequestVoteReply{true})
	if mcm.pcm.getServerState() != CANDIDATE {
		t.Fatal()
	}

	// s3 denies vote - should stay as candidate
	mcm.pcm.rpc("s3", &RpcRequestVoteReply{false})
	if mcm.pcm.getServerState() != CANDIDATE {
		t.Fatal()
	}

	// no more votes - election timeout causes a new election
	testCM_FollowerOrCandidate_StartsElectionOnElectionTimeout_Part2(t, mcm, mrs, testCurrentTerm+2)
}
