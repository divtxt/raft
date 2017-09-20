package candidate_test

import (
	"testing"

	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/config"
	"github.com/divtxt/raft/consensus/candidate"
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
	cvs, err := candidate.NewCandidateVolatileState(ci)
	if err != nil {
		t.Fatal(err)
	}

	// Initial state
	if cvs.ReceivedReplies != 1 {
		t.Fatal()
	}
	if cvs.ReceivedVotes != 1 {
		t.Fatal()
	}
	if cvs.QuorumSize != 3 {
		t.Fatal()
	}

	addVoteFrom := func(peerId ServerId, voteGranted bool) bool {
		r, err := cvs.AddVoteFrom(peerId, voteGranted)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}

	// Add a vote - no quorum yet
	if addVoteFrom(102, true) {
		t.Fatal()
	}
	if cvs.ReceivedReplies != 2 {
		t.Fatal()
	}
	if cvs.GotQuorumReplies() {
		t.Fatal()
	}

	// Duplicate vote - no error and no quorum yet
	if addVoteFrom(102, true) {
		t.Fatal()
	}
	if cvs.ReceivedReplies != 2 {
		t.Fatal()
	}
	if cvs.GotQuorumReplies() {
		t.Fatal()
	}

	// Add deny vote - no vote quorum, but have reply quorum
	if addVoteFrom(103, false) {
		t.Fatal()
	}
	if cvs.ReceivedReplies != 3 {
		t.Fatal()
	}
	if !cvs.GotQuorumReplies() {
		t.Fatal()
	}

	// Add another vote - should reach quorum
	if !addVoteFrom(104, true) {
		t.Fatal()
	}
	if cvs.ReceivedReplies != 4 {
		t.Fatal()
	}
	if !cvs.GotQuorumReplies() {
		t.Fatal()
	}

	// Last vote
	if !addVoteFrom(105, false) {
		t.Fatal()
	}
	if cvs.ReceivedReplies != 5 {
		t.Fatal()
	}
	if !cvs.GotQuorumReplies() {
		t.Fatal()
	}

	// Another duplicate vote - no error and stay at quorum
	if !addVoteFrom(103, true) {
		t.Fatal()
	}
	if cvs.ReceivedReplies != 5 {
		t.Fatal()
	}
	if !cvs.GotQuorumReplies() {
		t.Fatal()
	}

}

func TestCandidateVolatileState_3nodes(t *testing.T) {
	ci, err := config.NewClusterInfo([]ServerId{501, 502, 503}, 501)
	if err != nil {
		t.Fatal(err)
	}
	cvs, err := candidate.NewCandidateVolatileState(ci)
	if err != nil {
		t.Fatal(err)
	}
	if cvs.ReceivedReplies != 1 || cvs.ReceivedVotes != 1 || cvs.QuorumSize != 2 {
		t.Fatal()
	}

	addVoteFrom := func(peerId ServerId, voteGranted bool) bool {
		r, err := cvs.AddVoteFrom(peerId, voteGranted)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}

	if !addVoteFrom(503, true) {
		t.Fatal()
	}
	if !addVoteFrom(502, true) {
		t.Fatal()
	}
}

func TestCandidateVolatileState_VoteFromNonMemberIsAnError(t *testing.T) {
	ci, err := config.NewClusterInfo([]ServerId{501, 502, 503}, 501)
	if err != nil {
		t.Fatal(err)
	}
	cvs, err := candidate.NewCandidateVolatileState(ci)
	if err != nil {
		t.Fatal(err)
	}

	_, err = cvs.AddVoteFrom(504, true)
	if err.Error() != "CandidateVolatileState.AddVoteFrom(): unknown peer: 504" {
		t.Fatal(err)
	}
}
