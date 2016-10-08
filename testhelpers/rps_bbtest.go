package testhelpers

import (
	. "github.com/divtxt/raft"
	"testing"
)

// RaftPersistentState blackbox test.
// Send a RaftPersistentState in new / reset state.
func BlackboxTest_RaftPersistentState(
	t *testing.T,
	raftPersistentState RaftPersistentState,
) {
	// Initial data tests
	if raftPersistentState.GetCurrentTerm() != 0 {
		t.Fatal()
	}
	if raftPersistentState.GetVotedFor() != "" {
		t.Fatal()
	}

	// Set currentTerm to 0 is an error
	err := raftPersistentState.SetCurrentTerm(0)
	if err.Error() != "FATAL: attempt to set currentTerm to 0" {
		t.Fatal(err)
	}
	if raftPersistentState.GetCurrentTerm() != 0 {
		t.Fatal()
	}
	// Set votedFor while currentTerm is 0 is an error
	err = raftPersistentState.SetVotedFor("s1")
	if err.Error() != "FATAL: attempt to set votedFor while currentTerm is 0" {
		t.Fatal(err)
	}
	if raftPersistentState.GetCurrentTerm() != 0 {
		t.Fatal()
	}
	// Set currentTerm greater is ok, clears votedFor
	err = raftPersistentState.SetCurrentTerm(1)
	if err != nil {
		t.Fatal(err)
	}
	if raftPersistentState.GetCurrentTerm() != 1 {
		t.Fatal()
	}
	// Set votedFor of blank is an error
	err = raftPersistentState.SetVotedFor("")
	if err.Error() != "FATAL: attempt to set blank votedFor" {
		t.Fatal(err)
	}
	if raftPersistentState.GetVotedFor() != "" {
		t.Fatal()
	}
	// Set votedFor is ok
	err = raftPersistentState.SetVotedFor("s1")
	if err != nil {
		t.Fatal(err)
	}
	if raftPersistentState.GetVotedFor() != "s1" {
		t.Fatal()
	}
	// Set currentTerm greater is ok, clears votedFor
	err = raftPersistentState.SetCurrentTerm(4)
	if err != nil {
		t.Fatal(err)
	}
	if raftPersistentState.GetCurrentTerm() != 4 {
		t.Fatal()
	}
	if raftPersistentState.GetVotedFor() != "" {
		t.Fatal(raftPersistentState.GetVotedFor())
	}
	// Set votedFor while blank is ok
	err = raftPersistentState.SetVotedFor("s2")
	if err != nil {
		t.Fatal(err)
	}
	if raftPersistentState.GetVotedFor() != "s2" {
		t.Fatal()
	}
	// Set currentTerm same is ok, does not affect votedFor
	err = raftPersistentState.SetCurrentTerm(4)
	if err != nil {
		t.Fatal(err)
	}
	if raftPersistentState.GetCurrentTerm() != 4 {
		t.Fatal()
	}
	if raftPersistentState.GetVotedFor() != "s2" {
		t.Fatal()
	}
	// Set currentTerm less is an error
	err = raftPersistentState.SetCurrentTerm(3)
	if err.Error() != "FATAL: attempt to decrease currentTerm: 4 to 3" {
		t.Fatal(err)
	}
	if raftPersistentState.GetCurrentTerm() != 4 {
		t.Fatal()
	}
	// Set votedFor while not blank is an error
	err = raftPersistentState.SetVotedFor("s3")
	if err.Error() != "FATAL: attempt to change non-blank votedFor: s2 to s3" {
		t.Fatal(err)
	}
	if raftPersistentState.GetVotedFor() != "s2" {
		t.Fatal()
	}
}
