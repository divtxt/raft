package rps

import (
	"errors"
	"fmt"
	. "github.com/divtxt/raft"
	util "github.com/divtxt/raft/util"
	"os"
	"sync"
)

type rps struct {
	CurrentTerm TermNo   `json:"currentTerm"`
	VotedFor    ServerId `json:"votedFor"`
}

type JsonFileRaftPersistentState struct {
	mutex    *sync.Mutex
	filename string
	rps
}

// Json file implementation of RaftPersistentState.
//
// The returned instance is safe for access from multiple goroutines.
//
// The file access is NOT concurrency safe (from this or from another process).
//
// Writes to "filename" and also "filename.bak" using SafeWriteJsonToFile().
func NewJsonFileRaftPersistentState(filename string) (RaftPersistentState, error) {
	jfrps := &JsonFileRaftPersistentState{
		&sync.Mutex{}, filename, rps{0, ""},
	}

	err := util.ReadJsonFromFile(filename, &jfrps.rps)
	if err != nil {
		if os.IsNotExist(err) {
			jfrps.rps.CurrentTerm = 0
			jfrps.rps.VotedFor = ""
		} else {
			return nil, err
		}
	}

	return jfrps, nil
}

func (jfrps *JsonFileRaftPersistentState) writeToJsonFile() error {
	return util.SafeWriteJsonToFile(&jfrps.rps, jfrps.filename)
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
		jfrps.rps.VotedFor = ""
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
	if votedFor == "" {
		return errors.New("FATAL: attempt to set blank votedFor")
	}
	if jfrps.rps.VotedFor != "" {
		return fmt.Errorf(
			"FATAL: attempt to change non-blank votedFor: %v to %v", jfrps.rps.VotedFor, votedFor,
		)
	}
	jfrps.rps.VotedFor = votedFor
	return jfrps.writeToJsonFile()
}
