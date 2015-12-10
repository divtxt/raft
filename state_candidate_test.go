package raft

import (
	"testing"
)

func TestCandidateVolatileState(t *testing.T) {
	ci := NewClusterInfo(testAllServerIds, testThisServerId)
	cvs := newCandidateVolatileState(ci)

	// Initial state
	if cvs.receivedVotes != 1 {
		t.Fatal()
	}
	if cvs.requiredVotes != 3 {
		t.Fatal()
	}

	// Add a vote - no quorum yet
	if cvs.addVoteFrom("s2") {
		t.Fatal()
	}

	// Duplicate vote - no error and no quorum yet
	if cvs.addVoteFrom("s2") {
		t.Fatal()
	}

	// Add 2nd vote - should be at quorum
	if !cvs.addVoteFrom("s3") {
		t.Fatal()
	}

	// Add remaining votes - should stay at quorum
	if !cvs.addVoteFrom("s4") {
		t.Fatal()
	}
	if !cvs.addVoteFrom("s5") {
		t.Fatal()
	}
	// Another duplicate vote - no error and stay at quorum
	if !cvs.addVoteFrom("s3") {
		t.Fatal()
	}

}

func TestCandidateVolatileState_3nodes(t *testing.T) {
	ci := NewClusterInfo([]ServerId{"peer1", "peer2", "peer3"}, "peer1")
	cvs := newCandidateVolatileState(ci)
	if cvs.receivedVotes != 1 || cvs.requiredVotes != 2 {
		t.Fatal()
	}
	if !cvs.addVoteFrom("peer3") {
		t.Fatal()
	}
	if !cvs.addVoteFrom("peer2") {
		t.Fatal()
	}
}

func TestCandidateVolatileState_VoteFromNonMemberPanics(t *testing.T) {
	ci := NewClusterInfo([]ServerId{"peer1", "peer2", "peer3"}, "peer1")
	cvs := newCandidateVolatileState(ci)
	defer func() {
		if r := recover(); r != "candidateVolatileState.addVoteFrom(): unknown peer: peer4" {
			t.Error(r)
		}
	}()
	cvs.addVoteFrom("peer4")
	t.Fatal()
}
