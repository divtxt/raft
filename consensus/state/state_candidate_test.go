package consensus_state_test

import (
	"testing"

	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/config"
	consensus_state "github.com/divtxt/raft/consensus/state"
	"github.com/divtxt/raft/testdata"
)

func TestCandidateVolatileState(t *testing.T) {
	ci, err := config.NewClusterInfo(
		testdata.AllServerIds,
		testdata.ThisServerId,
	)
	if err != nil {
		t.Fatal(err)
	}
	cvs, err := consensus_state.NewCandidateVolatileState(ci)
	if err != nil {
		t.Fatal(err)
	}

	// Initial state
	if cvs.ReceivedVotes != 1 {
		t.Fatal()
	}
	if cvs.RequiredVotes != 3 {
		t.Fatal()
	}

	addVoteFrom := func(peerId ServerId) bool {
		r, err := cvs.AddVoteFrom(peerId)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}

	// Add a vote - no quorum yet
	if addVoteFrom(102) {
		t.Fatal()
	}

	// Duplicate vote - no error and no quorum yet
	if addVoteFrom(102) {
		t.Fatal()
	}

	// Add 2nd vote - should be at quorum
	if !addVoteFrom(103) {
		t.Fatal()
	}

	// Add remaining votes - should stay at quorum
	if !addVoteFrom(104) {
		t.Fatal()
	}
	if !addVoteFrom(105) {
		t.Fatal()
	}
	// Another duplicate vote - no error and stay at quorum
	if !addVoteFrom(103) {
		t.Fatal()
	}

}

func TestCandidateVolatileState_3nodes(t *testing.T) {
	ci, err := config.NewClusterInfo([]ServerId{501, 502, 503}, 501)
	if err != nil {
		t.Fatal(err)
	}
	cvs, err := consensus_state.NewCandidateVolatileState(ci)
	if err != nil {
		t.Fatal(err)
	}
	if cvs.ReceivedVotes != 1 || cvs.RequiredVotes != 2 {
		t.Fatal()
	}

	addVoteFrom := func(peerId ServerId) bool {
		r, err := cvs.AddVoteFrom(peerId)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}

	if !addVoteFrom(503) {
		t.Fatal()
	}
	if !addVoteFrom(502) {
		t.Fatal()
	}
}

func TestCandidateVolatileState_VoteFromNonMemberIsAnError(t *testing.T) {
	ci, err := config.NewClusterInfo([]ServerId{501, 502, 503}, 501)
	if err != nil {
		t.Fatal(err)
	}
	cvs, err := consensus_state.NewCandidateVolatileState(ci)
	if err != nil {
		t.Fatal(err)
	}

	_, err = cvs.AddVoteFrom(504)
	if err.Error() != "CandidateVolatileState.AddVoteFrom(): unknown peer: 504" {
		t.Fatal(err)
	}
}
