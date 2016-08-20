package log

import (
	"errors"
	"fmt"
	. "github.com/divtxt/raft"
)

// In-memory implementation of locking as raft.Log

type InMemoryLog struct {
	entries                  []LogEntry
	MaxEntriesPerAppendEntry uint64
}

func NewInMemoryLog(maxEntriesPerAppendEntry uint64) *InMemoryLog {
	// FIXME: move maxEntriesPerAppendEntry out of here?!
	if maxEntriesPerAppendEntry <= 0 {
		panic("maxEntriesPerAppendEntry must be greater than zero")
	}
	entries := []LogEntry{}
	iml := &InMemoryLog{entries, maxEntriesPerAppendEntry}
	return iml
}

func (iml *InMemoryLog) GetIndexOfLastEntry() (LogIndex, error) {
	return LogIndex(len(iml.entries)), nil
}

func (iml *InMemoryLog) GetTermAtIndex(li LogIndex) (TermNo, error) {
	if li == 0 {
		return 0, errors.New("GetTermAtIndex(): li=0")
	}
	if li > LogIndex(len(iml.entries)) {
		return 0, fmt.Errorf(
			"GetTermAtIndex(): li=%v > iole=%v", li, len(iml.entries),
		)
	}
	return iml.entries[li-1].TermNo, nil
}

func (iml *InMemoryLog) GetEntriesAfterIndex(afterLogIndex LogIndex) ([]LogEntry, error) {
	iole, err := iml.GetIndexOfLastEntry()
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

	if numEntriesToGet > iml.MaxEntriesPerAppendEntry {
		numEntriesToGet = iml.MaxEntriesPerAppendEntry
	}

	logEntries := make([]LogEntry, numEntriesToGet)
	var i uint64 = 0
	nextIndexToGet := afterLogIndex + 1

	for i < numEntriesToGet {
		logEntries[i] = iml.entries[nextIndexToGet-1]
		i++
		nextIndexToGet++
	}

	return logEntries, nil
}

func (iml *InMemoryLog) SetEntriesAfterIndex(li LogIndex, entries []LogEntry) error {
	iole, err := iml.GetIndexOfLastEntry()
	if err != nil {
		return err
	}
	if iole < li {
		return fmt.Errorf("InMemoryLog: setEntriesAfterIndex(%d, ...) but iole=%d", li, iole)
	}
	// delete entries after index
	if iole > li {
		iml.entries = iml.entries[:li]
	}
	// append entries
	iml.entries = append(iml.entries, entries...)
	return nil
}

func (iml *InMemoryLog) AppendEntry(logEntry LogEntry) error {
	iml.entries = append(iml.entries, logEntry)
	return nil
}
