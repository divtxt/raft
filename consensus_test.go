package raft

import (
	"testing"
)

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

// #5.2-p1s2: When servers start up, they begin as followers
func TestCMStartsAsFollower(t *testing.T) {
	cm := NewConsensusModule(newIMPSWithCurrentTerm(TEST_CURRENT_TERM), nil)

	if cm == nil {
		t.Fatal()
	}
	if cm.serverMode != FOLLOWER {
		t.Fatal()
	}
}
