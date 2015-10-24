package raft

import (
	"testing"
)

// #5.2-p3s1: A candidate wins an election if it receives votes from a
// majority of the servers in the full cluster for the same term.
func TestRpcRVR_CandidateWinsElectionIfItReceivesMajorityOfVotes(t *testing.T) {
	terms := testLogTerms_Figure7LeaderLine()
	mcm, mrs := setupManagedConsensusModuleR2(t, terms)

	testCMFollowerStartsElectionOnElectionTimeout(t, mcm, mrs)

	if mcm.pcm.getServerState() != CANDIDATE {
		t.Fatal()
	}

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
func TestRpcRVR_StartNewElectionOnElectionTimeout(t *testing.T) {
	terms := testLogTerms_Figure7LeaderLine()
	mcm, mrs := setupManagedConsensusModuleR2(t, terms)

	testCMFollowerStartsElectionOnElectionTimeout(t, mcm, mrs)

	if mcm.pcm.getServerState() != CANDIDATE {
		t.Fatal()
	}

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
	testCMFollowerStartsElectionOnElectionTimeout_Part2(t, mcm, mrs, testCurrentTerm+2)
	//
	// time.Sleep(cm.currentElectionTimeout)
	// if cm.persistentState.GetCurrentTerm() != testCurrentTerm+2 {
	//     t.Fatal()
	// }
	// if cm.getServerState() != CANDIDATE {
	//     t.Fatal()
	// }
	// // candidate has voted for itself
	// if cm.persistentState.GetVotedFor() != testServerId {
	//     t.Fatal()
	// }
}
