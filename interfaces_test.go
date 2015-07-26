package raft

import (
	"testing"
)

// Blackbox test
// Send a PersistentState in new / reset state.
func PersistentStateBlackboxTest(t *testing.T, persistentState PersistentState) {
	// Initial data tests
	if persistentState.GetCurrentTerm() != 0 {
		t.Fatal()
	}

	// Set & get tests
	persistentState.SetCurrentTerm(1)
	if persistentState.GetCurrentTerm() != 1 {
		t.Fatal()
	}
	persistentState.SetCurrentTerm(4)
	if persistentState.GetCurrentTerm() != 4 {
		t.Fatal()
	}
}

// In-memory implementation of PersistentState - meant only for tests
type inMemoryPersistentState struct {
	currentTerm TermNo
}

func (imps *inMemoryPersistentState) GetCurrentTerm() TermNo {
	return imps.currentTerm
}

func (imps *inMemoryPersistentState) SetCurrentTerm(currentTerm TermNo) {
	imps.currentTerm = currentTerm
}

func newIMPSWithCurrentTerm(currentTerm TermNo) *inMemoryPersistentState {
	ps := new(inMemoryPersistentState)
	ps.currentTerm = currentTerm
	return ps
}

// Run the blackbox test on inMemoryPersistentState
func TestInMemoryPersistentState(t *testing.T) {
	imps := newIMPSWithCurrentTerm(0)
	PersistentStateBlackboxTest(t, imps)
}
