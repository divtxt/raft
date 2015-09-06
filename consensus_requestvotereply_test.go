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
	defer cm.StopAsync()

	testCMFollowerStartsElectionOnElectionTimeout(t, cm, mrs)

	if cm.GetServerState() != CANDIDATE {
		t.Fatal()
	}

	// s2 grants vote - should stay as candidate
	cm.ProcessRpcAsync("s2", &RpcRequestVoteReply{true})
	time.Sleep(1 * time.Millisecond)
	if cm.GetServerState() != CANDIDATE {
		t.Fatal()
	}

	// s3 denies vote - should stay as candidate
	cm.ProcessRpcAsync("s3", &RpcRequestVoteReply{false})
	time.Sleep(1 * time.Millisecond)
	if cm.GetServerState() != CANDIDATE {
		t.Fatal()
	}

	// s4 grants vote - should become leader
	cm.ProcessRpcAsync("s4", &RpcRequestVoteReply{true})
	time.Sleep(1 * time.Millisecond)
	if cm.GetServerState() != LEADER {
		t.Fatal()
	}

	// s5 grants vote - should stay leader
	cm.ProcessRpcAsync("s5", &RpcRequestVoteReply{true})
	time.Sleep(1 * time.Millisecond)
	if cm.GetServerState() != LEADER {
		t.Fatal()
	}
}
