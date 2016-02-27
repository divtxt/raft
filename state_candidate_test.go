package raft

import (
	"testing"
)

func TestCandidateVolatileState(t *testing.T) {
	ci, err := NewClusterInfo(testAllServerIds, testThisServerId)
	if err != nil {
		t.Fatal(err)
	}
	cvs := newCandidateVolatileState(ci)

	// Initial state
	if cvs.receivedVotes != 1 {
		t.Fatal()
	}
	if cvs.requiredVotes != 3 {
		t.Fatal()
	}

	addVoteFrom := func(peerId ServerId) bool {
		r, err := cvs.addVoteFrom(peerId)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}

	// Add a vote - no quorum yet
	if addVoteFrom("s2") {
		t.Fatal()
	}

	// Duplicate vote - no error and no quorum yet
	if addVoteFrom("s2") {
		t.Fatal()
	}

	// Add 2nd vote - should be at quorum
	if !addVoteFrom("s3") {
		t.Fatal()
	}

	// Add remaining votes - should stay at quorum
	if !addVoteFrom("s4") {
		t.Fatal()
	}
	if !addVoteFrom("s5") {
		t.Fatal()
	}
	// Another duplicate vote - no error and stay at quorum
	if !addVoteFrom("s3") {
		t.Fatal()
	}

}

func TestCandidateVolatileState_3nodes(t *testing.T) {
	ci, err := NewClusterInfo([]ServerId{"peer1", "peer2", "peer3"}, "peer1")
	if err != nil {
		t.Fatal(err)
	}
	cvs := newCandidateVolatileState(ci)
	if cvs.receivedVotes != 1 || cvs.requiredVotes != 2 {
		t.Fatal()
	}

	addVoteFrom := func(peerId ServerId) bool {
		r, err := cvs.addVoteFrom(peerId)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}

	if !addVoteFrom("peer3") {
		t.Fatal()
	}
	if !addVoteFrom("peer2") {
		t.Fatal()
	}
}

func TestCandidateVolatileState_VoteFromNonMemberIsAnError(t *testing.T) {
	ci, err := NewClusterInfo([]ServerId{"peer1", "peer2", "peer3"}, "peer1")
	if err != nil {
		t.Fatal(err)
	}
	cvs := newCandidateVolatileState(ci)

	_, err = cvs.addVoteFrom("peer4")
	if err.Error() != "candidateVolatileState.addVoteFrom(): unknown peer: peer4" {
		t.Fatal(err)
	}
}
