package lasm

import (
	"errors"
	"fmt"
	. "github.com/divtxt/raft"
	"strconv"
)

// Dummy in-memory implementation of LogAndStateMachine.
// Does not provide any useful state or commands. Meant only for tests,
type DummyInMemoryLasm struct {
	entries                  []LogEntry
	_commitIndex             LogIndex
	MaxEntriesPerAppendEntry uint64
}

func (diml *DummyInMemoryLasm) GetIndexOfLastEntry() (LogIndex, error) {
	return LogIndex(len(diml.entries)), nil
}

func (diml *DummyInMemoryLasm) GetTermAtIndex(li LogIndex) (TermNo, error) {
	if li == 0 {
		return 0, errors.New("GetTermAtIndex(): li=0")
	}
	if li > LogIndex(len(diml.entries)) {
		return 0, fmt.Errorf(
			"GetTermAtIndex(): li=%v > iole=%v", li, len(diml.entries),
		)
	}
	return diml.entries[li-1].TermNo, nil
}

func (diml *DummyInMemoryLasm) SetEntriesAfterIndex(
	li LogIndex,
	entries []LogEntry,
) error {
	if li < diml._commitIndex {
		return fmt.Errorf(
			"DummyInMemoryLasm: setEntriesAfterIndex(%d, ...) but commitIndex=%d",
			li,
			diml._commitIndex,
		)
	}
	iole, err := diml.GetIndexOfLastEntry()
	if err != nil {
		return err
	}
	if iole < li {
		return fmt.Errorf("DummyInMemoryLasm: setEntriesAfterIndex(%d, ...) but iole=%d", li, iole)
	}
	// delete entries after index
	if iole > li {
		diml.entries = diml.entries[:li]
	}
	// append entries
	diml.entries = append(diml.entries, entries...)
	return nil
}

func (diml *DummyInMemoryLasm) AppendEntry(
	termNo TermNo,
	rawCommand interface{},
) (interface{}, error) {
	switch rawCommand := rawCommand.(type) {
	case string:
		entry := LogEntry{termNo, []byte(rawCommand)}
		diml.entries = append(diml.entries, entry)
	default:
		// FIXME: test
		return fmt.Errorf("oops! rawCommand %#v of unknown type: %T", rawCommand, rawCommand), nil
	}
	return DummyInMemoryLasm_AppendEntry_Ok, nil
}

const (
	DummyInMemoryLasm_AppendEntry_Ok = "We accept your offering!"
)

func (diml *DummyInMemoryLasm) GetEntriesAfterIndex(
	afterLogIndex LogIndex,
) ([]LogEntry, error) {
	iole, err := diml.GetIndexOfLastEntry()
	if err != nil {
		return nil, err
	}

	if iole < afterLogIndex {
		return nil, fmt.Errorf(
			"afterLogIndex=%v is > iole=%v",
			afterLogIndex,
			iole,
		)
	}

	var numEntriesToGet uint64 = uint64(iole - afterLogIndex)

	// Short-circuit allocation for common case
	if numEntriesToGet == 0 {
		return []LogEntry{}, nil
	}

	if numEntriesToGet > diml.MaxEntriesPerAppendEntry {
		numEntriesToGet = diml.MaxEntriesPerAppendEntry
	}

	logEntries := make([]LogEntry, numEntriesToGet)
	var i uint64 = 0
	nextIndexToGet := afterLogIndex + 1

	for i < numEntriesToGet {
		logEntries[i] = diml.entries[nextIndexToGet-1]
		i++
		nextIndexToGet++
	}

	return logEntries, nil
}

func (diml *DummyInMemoryLasm) CommitIndexChanged(commitIndex LogIndex) error {
	if commitIndex < diml._commitIndex {
		return fmt.Errorf(
			"DummyInMemoryLasm: CommitIndexChanged(%d) is < current commitIndex=%d",
			commitIndex,
			diml._commitIndex,
		)
	}
	iole, err := diml.GetIndexOfLastEntry()
	if err != nil {
		return err
	}
	if commitIndex > iole {
		return fmt.Errorf(
			"DummyInMemoryLasm: CommitIndexChanged(%d) is > iole=%d",
			commitIndex,
			iole,
		)
	}
	diml._commitIndex = commitIndex
	return nil
}

func NewDummyInMemoryLasmWithDummyCommands(
	logTerms []TermNo,
	maxEntriesPerAppendEntry uint64,
) *DummyInMemoryLasm {
	if maxEntriesPerAppendEntry <= 0 {
		panic("maxEntriesPerAppendEntry must be greater than zero")
	}
	entries := []LogEntry{}
	for i, term := range logTerms {
		entries = append(entries, LogEntry{term, Command("c" + strconv.Itoa(i+1))})
	}
	diml := &DummyInMemoryLasm{
		entries,
		0,
		maxEntriesPerAppendEntry,
	}
	return diml
}

func (diml *DummyInMemoryLasm) GetCommitIndex() LogIndex {
	return diml._commitIndex
}
