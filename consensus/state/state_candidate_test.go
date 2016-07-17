package consensus_state_test

import (
	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/config"
	consensus_state "github.com/divtxt/raft/consensus/state"
	"github.com/divtxt/raft/testdata"
	"testing"
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
	ci, err := config.NewClusterInfo([]ServerId{"peer1", "peer2", "peer3"}, "peer1")
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

	if !addVoteFrom("peer3") {
		t.Fatal()
	}
	if !addVoteFrom("peer2") {
		t.Fatal()
	}
}

func TestCandidateVolatileState_VoteFromNonMemberIsAnError(t *testing.T) {
	ci, err := config.NewClusterInfo([]ServerId{"peer1", "peer2", "peer3"}, "peer1")
	if err != nil {
		t.Fatal(err)
	}
	cvs, err := consensus_state.NewCandidateVolatileState(ci)
	if err != nil {
		t.Fatal(err)
	}

	_, err = cvs.AddVoteFrom("peer4")
	if err.Error() != "CandidateVolatileState.AddVoteFrom(): unknown peer: peer4" {
		t.Fatal(err)
	}
}
