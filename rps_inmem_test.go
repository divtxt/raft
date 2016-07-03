package raft

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

// In-memory implementation of RaftPersistentState - meant only for tests

type inMemoryRaftPersistentState struct {
	mutex       *sync.Mutex
	currentTerm TermNo
	votedFor    ServerId
}

func (imps *inMemoryRaftPersistentState) GetCurrentTerm() TermNo {
	imps.mutex.Lock()
	defer imps.mutex.Unlock()
	return imps.currentTerm
}

func (imps *inMemoryRaftPersistentState) GetVotedFor() ServerId {
	imps.mutex.Lock()
	defer imps.mutex.Unlock()
	return imps.votedFor
}

func (imps *inMemoryRaftPersistentState) SetCurrentTerm(currentTerm TermNo) error {
	imps.mutex.Lock()
	defer imps.mutex.Unlock()
	if currentTerm == 0 {
		return errors.New("FATAL: attempt to set currentTerm to 0")
	}
	if currentTerm < imps.currentTerm {
		return fmt.Errorf("FATAL: attempt to decrease currentTerm: %v to %v", imps.currentTerm, currentTerm)
	}
	if currentTerm > imps.currentTerm {
		imps.votedFor = ""
	}
	imps.currentTerm = currentTerm
	return nil
}

func (imps *inMemoryRaftPersistentState) SetVotedFor(votedFor ServerId) error {
	imps.mutex.Lock()
	defer imps.mutex.Unlock()
	if imps.currentTerm == 0 {
		return errors.New("FATAL: attempt to set votedFor while currentTerm is 0")
	}
	if votedFor == "" {
		return errors.New("FATAL: attempt to set blank votedFor")
	}
	if imps.votedFor != "" {
		return fmt.Errorf("FATAL: attempt to change non-blank votedFor: %v to %v", imps.votedFor, votedFor)
	}
	imps.votedFor = votedFor
	return nil
}

func newIMPSWithCurrentTerm(currentTerm TermNo) *inMemoryRaftPersistentState {
	return &inMemoryRaftPersistentState{&sync.Mutex{}, currentTerm, ""}
}

// Run the blackbox test on inMemoryRaftPersistentState
func TestInMemoryRaftPersistentState(t *testing.T) {
	imps := newIMPSWithCurrentTerm(0)
	BlackboxTest_RaftPersistentState(t, imps)
}
