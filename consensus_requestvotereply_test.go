package raft

import (
	"testing"
	"time"
)

// #5.2-p3s1: A candidate wins an election if it receives votes from a
// majority of the servers in the full cluster for the same term.
func TestRpcRVR_CandidateWinsElectionIfItReceivesMajorityOfVotes(t *testing.T) {
	terms := testLogTerms_Figure7LeaderLine()
	cm, mrs := setupTestFollowerR2(t, terms)
	defer cm.stopAndCheckError()

	testCMFollowerStartsElectionOnElectionTimeout(t, cm, mrs)

	if cm.GetServerState() != CANDIDATE {
		t.Fatal()
	}

	// s2 grants vote - should stay as candidate
	cm.ProcessRpcAsync("s2", &RpcRequestVoteReply{true})
	time.Sleep(testSleepToLetGoroutineRun)
	if cm.GetServerState() != CANDIDATE {
		t.Fatal()
	}

	// s3 denies vote - should stay as candidate
	cm.ProcessRpcAsync("s3", &RpcRequestVoteReply{false})
	time.Sleep(testSleepToLetGoroutineRun)
	if cm.GetServerState() != CANDIDATE {
		t.Fatal()
	}

	// s4 grants vote - should become leader
	cm.ProcessRpcAsync("s4", &RpcRequestVoteReply{true})
	time.Sleep(testSleepToLetGoroutineRun)
	if cm.GetServerState() != LEADER {
		t.Fatal()
	}

	// s5 grants vote - should stay leader
	cm.ProcessRpcAsync("s5", &RpcRequestVoteReply{true})
	time.Sleep(testSleepToLetGoroutineRun)
	if cm.GetServerState() != LEADER {
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
	cm, mrs := setupTestFollowerR2(t, terms)
	defer cm.stopAndCheckError()

	testCMFollowerStartsElectionOnElectionTimeout(t, cm, mrs)

	if cm.GetServerState() != CANDIDATE {
		t.Fatal()
	}

	// s2 grants vote - should stay as candidate
	cm.ProcessRpcAsync("s2", &RpcRequestVoteReply{true})
	time.Sleep(testSleepToLetGoroutineRun)
	if cm.GetServerState() != CANDIDATE {
		t.Fatal()
	}

	// s3 denies vote - should stay as candidate
	cm.ProcessRpcAsync("s3", &RpcRequestVoteReply{false})
	time.Sleep(testSleepToLetGoroutineRun)
	if cm.GetServerState() != CANDIDATE {
		t.Fatal()
	}

	// no more votes - election timeout causes a new election
	testCMFollowerStartsElectionOnElectionTimeout_Part2(t, cm, mrs, testCurrentTerm+2)
	//
	// time.Sleep(cm.currentElectionTimeout)
	// if cm.persistentState.GetCurrentTerm() != testCurrentTerm+2 {
	//     t.Fatal()
	// }
	// if cm.GetServerState() != CANDIDATE {
	//     t.Fatal()
	// }
	// // candidate has voted for itself
	// if cm.persistentState.GetVotedFor() != testServerId {
	//     t.Fatal()
	// }
}
