package rps

import (
	"errors"
	"fmt"
	"sync"

	. "github.com/divtxt/raft"
)

// In-memory implementation of RaftPersistentState
type InMemoryRaftPersistentState struct {
	mutex       *sync.Mutex
	currentTerm TermNo
	votedFor    ServerId
}

func (imps *InMemoryRaftPersistentState) GetCurrentTerm() TermNo {
	imps.mutex.Lock()
	defer imps.mutex.Unlock()
	return imps.currentTerm
}

func (imps *InMemoryRaftPersistentState) GetVotedFor() ServerId {
	imps.mutex.Lock()
	defer imps.mutex.Unlock()
	return imps.votedFor
}

func (imps *InMemoryRaftPersistentState) SetCurrentTerm(currentTerm TermNo) error {
	imps.mutex.Lock()
	defer imps.mutex.Unlock()
	if currentTerm == 0 {
		return errors.New("FATAL: attempt to set currentTerm to 0")
	}
	if currentTerm < imps.currentTerm {
		return fmt.Errorf(
			"FATAL: attempt to decrease currentTerm: %v to %v", imps.currentTerm, currentTerm,
		)
	}
	if currentTerm > imps.currentTerm {
		imps.votedFor = 0
	}
	imps.currentTerm = currentTerm
	return nil
}

func (imps *InMemoryRaftPersistentState) SetVotedFor(votedFor ServerId) error {
	imps.mutex.Lock()
	defer imps.mutex.Unlock()
	if imps.currentTerm == 0 {
		return errors.New("FATAL: attempt to set votedFor while currentTerm is 0")
	}
	if votedFor == 0 {
		return errors.New("FATAL: attempt to set votedFor to 0")
	}
	if imps.votedFor != 0 {
		return fmt.Errorf(
			"FATAL: attempt to change non-zero votedFor: %v to %v", imps.votedFor, votedFor,
		)
	}
	imps.votedFor = votedFor
	return nil
}

func NewIMPSWithCurrentTerm(currentTerm TermNo) *InMemoryRaftPersistentState {
	return &InMemoryRaftPersistentState{&sync.Mutex{}, currentTerm, 0}
}
