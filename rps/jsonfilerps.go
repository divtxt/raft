package rps

import (
	"errors"
	"fmt"
	"os"
	"sync"

	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/fileutil"
)

type rps struct {
	CurrentTerm TermNo   `json:"currentTerm"`
	VotedFor    ServerId `json:"votedFor"`
}

type JsonFileRaftPersistentState struct {
	mutex *sync.Mutex
	ajf   fileutil.AtomicJsonFile
	rps
}

// Json file implementation of RaftPersistentState.
//
// The state is initialized by reading the current values in the given AtomicJsonFile.
// If the file does not exist, the values are initialized to default values.
// However, the file is not actually written until a setter call.
//
// Every setter will synchronously write to the underlying json file.
//
// The writes are done without reading the current values and the file is never read
// after initialization. This means that concurrent writes by another instance, method
// or process is unsafe while this returned instance is in use.
// The caller is responsible for ensuring safe/exclusive access to the underlying file.
//
// The returned instance is safe for access from multiple goroutines.
//
func NewJsonFileRaftPersistentState(ajf fileutil.AtomicJsonFile) (*JsonFileRaftPersistentState, error) {
	jfrps := &JsonFileRaftPersistentState{
		&sync.Mutex{}, ajf, rps{0, 0},
	}

	err := ajf.Read(&jfrps.rps)
	if err != nil {
		if os.IsNotExist(err) {
			jfrps.rps.CurrentTerm = 0
			jfrps.rps.VotedFor = 0
		} else {
			return nil, err
		}
	}

	return jfrps, nil
}

func (jfrps *JsonFileRaftPersistentState) writeToJsonFile() error {
	return jfrps.ajf.Write(&jfrps.rps)
}

func (jfrps *JsonFileRaftPersistentState) GetCurrentTerm() TermNo {
	jfrps.mutex.Lock()
	defer jfrps.mutex.Unlock()
	return jfrps.rps.CurrentTerm
}

func (jfrps *JsonFileRaftPersistentState) GetVotedFor() ServerId {
	jfrps.mutex.Lock()
	defer jfrps.mutex.Unlock()
	return jfrps.rps.VotedFor
}

func (jfrps *JsonFileRaftPersistentState) SetCurrentTerm(currentTerm TermNo) error {
	jfrps.mutex.Lock()
	defer jfrps.mutex.Unlock()
	if currentTerm == 0 {
		return errors.New("FATAL: attempt to set currentTerm to 0")
	}
	if currentTerm < jfrps.rps.CurrentTerm {
		return fmt.Errorf(
			"FATAL: attempt to decrease currentTerm: %v to %v", jfrps.CurrentTerm, currentTerm,
		)
	}
	if currentTerm > jfrps.rps.CurrentTerm {
		jfrps.rps.VotedFor = 0
	}
	jfrps.rps.CurrentTerm = currentTerm
	return jfrps.writeToJsonFile()
}

func (jfrps *JsonFileRaftPersistentState) SetVotedFor(votedFor ServerId) error {
	jfrps.mutex.Lock()
	defer jfrps.mutex.Unlock()
	if jfrps.rps.CurrentTerm == 0 {
		return errors.New("FATAL: attempt to set votedFor while currentTerm is 0")
	}
	if votedFor == 0 {
		return errors.New("FATAL: attempt to set votedFor to 0")
	}
	if jfrps.rps.VotedFor != 0 {
		return fmt.Errorf(
			"FATAL: attempt to change non-zero votedFor: %v to %v", jfrps.rps.VotedFor, votedFor,
		)
	}
	jfrps.rps.VotedFor = votedFor
	return jfrps.writeToJsonFile()
}
