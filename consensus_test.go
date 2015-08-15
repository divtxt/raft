package raft

import (
	"reflect"
	"testing"
	"time"
)

const (
	testServerId = "s1"
	// Note: value for tests based on Figure 7
	testCurrentTerm = 8
)

var testPeerIds = []ServerId{"s2", "s3", "s4", "s5"}

func setupTestFollower(t *testing.T, logTerms []TermNo) *ConsensusModule {
	cm, _ := setupTestFollowerR2(t, logTerms)
	return cm
}

func setupTestFollowerR2(
	t *testing.T,
	logTerms []TermNo,
) (*ConsensusModule, *mockRpcSender) {
	ps := newIMPSWithCurrentTerm(testCurrentTerm)
	imle := newIMLEWithDummyCommands(logTerms)
	mrs := newMockRpcSender()
	ts := TimeSettings{5 * time.Millisecond, 50 * time.Millisecond}
	cm := NewConsensusModule(ps, imle, mrs, testServerId, testPeerIds, ts)
	if cm == nil {
		t.Fatal()
	}
	return cm, mrs
}

// #5.2-p1s2: When servers start up, they begin as followers
func TestCMStartsAsFollower(t *testing.T) {
	cm := setupTestFollower(t, nil)
	defer cm.StopAsync()

	if cm.GetServerState() != FOLLOWER {
		t.Fatal()
	}
}

func TestCMStop(t *testing.T) {
	cm := setupTestFollower(t, nil)

	if cm.IsStopped() {
		t.Error()
	}

	cm.StopAsync()
	time.Sleep(1 * time.Millisecond)

	if !cm.IsStopped() {
		t.Error()
	}
}

// #5.2-p1s5: If a follower receives no communication over a period of time
// called the election timeout, then it assumes there is no viable leader
// and begins an election to choose a new leader.
// #5.2-p2s1: To begin an election, a follower increments its current term
// and transitions to candidate state.
// #5.2-p2s2: It then votes for itself and issues RequestVote RPCs in parallel
// to each of the other servers in the cluster.
func TestCMFollowerStartsElectionOnElectionTimeout(t *testing.T) {
	cm, mrs := setupTestFollowerR2(t, nil)
	defer cm.StopAsync()

	if cm.GetServerState() != FOLLOWER {
		t.Fatal()
	}
	if cm.persistentState.GetVotedFor() != "" {
		t.Fatal()
	}

	// Test that a tick before election timeout causes no state change.
	time.Sleep(1 * time.Millisecond)
	if cm.persistentState.GetCurrentTerm() != testCurrentTerm {
		t.Fatal()
	}
	if cm.GetServerState() != FOLLOWER {
		t.Fatal()
	}

	// Test that election timeout causes a new election
	time.Sleep(50 * time.Millisecond)
	if cm.persistentState.GetCurrentTerm() != testCurrentTerm+1 {
		t.Fatal()
	}
	if cm.GetServerState() != CANDIDATE {
		t.Fatal()
	}
	// candidate has voted for itself
	if cm.persistentState.GetVotedFor() != testServerId {
		t.Fatal()
	}
	// candidate has issued RequestVote RPCs to all other servers.
	expectedIds := make(map[ServerId]bool)
	for _, peerId := range testPeerIds {
		expectedIds[peerId] = true
	}

	sentRpcs := mrs.getAllSortedByToServer()

	if len(sentRpcs) != len(testPeerIds) {
		t.Error()
	}

	expectedRpc := &RpcRequestVote{testCurrentTerm + 1, 0, 0}

	for i, peerId := range testPeerIds {
		sentRpc := sentRpcs[i]
		if sentRpc.toServer != peerId {
			t.Error()
		}
		if !reflect.DeepEqual(sentRpc.rpc, expectedRpc) {
			t.Fatal(sentRpc.rpc, expectedRpc)
		}
	}
}
