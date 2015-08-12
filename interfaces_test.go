package raft

import (
	"sync"
	"testing"
)

// Blackbox test
// Send a PersistentState in new / reset state.
func PersistentStateBlackboxTest(t *testing.T, persistentState PersistentState) {
	// Initial data tests
	if persistentState.GetCurrentTerm() != 0 {
		t.Fatal()
	}
	if persistentState.GetVotedFor() != "" {
		t.Fatal()
	}

	// Set & get tests
	persistentState.SetCurrentTermAndVotedFor(1, "s1")
	if persistentState.GetCurrentTerm() != 1 {
		t.Fatal()
	}
	if persistentState.GetVotedFor() != "s1" {
		t.Fatal()
	}
	persistentState.SetCurrentTermAndVotedFor(4, "s2")
	if persistentState.GetCurrentTerm() != 4 {
		t.Fatal()
	}
	if persistentState.GetVotedFor() != "s2" {
		t.Fatal()
	}
}

// In-memory implementation of PersistentState - meant only for tests
type inMemoryPersistentState struct {
	mutex       *sync.Mutex
	currentTerm TermNo
	votedFor    ServerId
}

func (imps *inMemoryPersistentState) GetCurrentTerm() TermNo {
	imps.mutex.Lock()
	defer imps.mutex.Unlock()
	return imps.currentTerm
}

func (imps *inMemoryPersistentState) GetVotedFor() ServerId {
	imps.mutex.Lock()
	defer imps.mutex.Unlock()
	return imps.votedFor
}

func (imps *inMemoryPersistentState) SetCurrentTermAndVotedFor(
	currentTerm TermNo,
	votedFor ServerId,
) {
	imps.mutex.Lock()
	defer imps.mutex.Unlock()
	imps.currentTerm = currentTerm
	imps.votedFor = votedFor
}

func newIMPSWithCurrentTerm(currentTerm TermNo) *inMemoryPersistentState {
	return &inMemoryPersistentState{&sync.Mutex{}, currentTerm, ""}
}

// Run the blackbox test on inMemoryPersistentState
func TestInMemoryPersistentState(t *testing.T) {
	imps := newIMPSWithCurrentTerm(0)
	PersistentStateBlackboxTest(t, imps)
}
