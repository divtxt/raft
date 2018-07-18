package log

import (
	"errors"
	"fmt"

	. "github.com/divtxt/raft"
)

// InMemoryLog is an in-memory raft Log.
type InMemoryLog struct {
	indexOfFirstEntry LogIndex
	entries           []LogEntry
	maxEntries        uint64
}

func NewInMemoryLog(maxEntries uint64) *InMemoryLog {
	if maxEntries <= 0 {
		panic("maxEntries must be greater than zero")
	}
	entries := []LogEntry{}
	iml := &InMemoryLog{
		1,
		entries,
		maxEntries,
	}
	return iml
}

func (iml *InMemoryLog) GetIndexOfFirstEntry() (LogIndex, error) {
	return iml.indexOfFirstEntry, nil
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
	if afterLogIndex+1 < iml.indexOfFirstEntry {
		return nil, ErrIndexBeforeFirstEntry
	}

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

	// Short-circuit allocation for no entries to return
	if numEntriesToGet == 0 {
		return []LogEntry{}, nil
	}

	if numEntriesToGet > iml.maxEntries {
		numEntriesToGet = iml.maxEntries
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
	if li+1 < iml.indexOfFirstEntry {
		return ErrIndexBeforeFirstEntry
	}
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

func (iml *InMemoryLog) DiscardEntriesBeforeIndex(li LogIndex) error {
	if li < iml.indexOfFirstEntry {
		return ErrIndexBeforeFirstEntry
	}
	iml.indexOfFirstEntry = li
	// FIXME: actually throw away entries!
	return nil
}

func (iml *InMemoryLog) AppendEntry(logEntry LogEntry) (LogIndex, error) {
	// return fmt.Errorf("InMemoryLog: EEEE: %v", logEntry)

	iml.entries = append(iml.entries, logEntry)
	return LogIndex(len(iml.entries)), nil
}
