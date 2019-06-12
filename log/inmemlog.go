package log

import (
	"errors"
	"fmt"
	"sync"

	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/logindex"
)

// InMemoryLog is an in-memory raft Log.
type InMemoryLog struct {
	// TODO: see if we can use RWMutex
	lock             *sync.Mutex
	indexOfLastEntry *logindex.WatchedIndex
	lastCompacted    LogIndex
	entries          []LogEntry
	maxEntries       uint64
}

// Test that InMemoryLog implements the Log interface
var _ Log = (*InMemoryLog)(nil)

func NewInMemoryLog(maxEntries uint64) (*InMemoryLog, error) {
	if maxEntries <= 0 {
		return nil, fmt.Errorf("maxEntries =%v must be greater than zero", maxEntries)
	}
	entries := []LogEntry{}
	lock := &sync.Mutex{}
	iml := &InMemoryLog{
		lock,
		logindex.NewWatchedIndex(lock),
		0,
		entries,
		maxEntries,
	}
	return iml, nil
}

func (iml *InMemoryLog) GetLastCompacted() LogIndex {
	iml.lock.Lock()
	defer iml.lock.Unlock()

	return iml.lastCompacted
}

func (iml *InMemoryLog) GetIndexOfLastEntry() LogIndex {
	// Get() will lock the lock
	return iml.indexOfLastEntry.Get()
}

func (iml *InMemoryLog) GetIndexOfLastEntryWatchable() WatchableIndex {
	return iml.indexOfLastEntry
}

func (iml *InMemoryLog) GetTermAtIndex(li LogIndex) (TermNo, error) {
	iml.lock.Lock()
	defer iml.lock.Unlock()

	if li == 0 {
		return 0, errors.New("GetTermAtIndex(): li=0")
	}

	if li > iml.indexOfLastEntry.UnsafeGet() {
		return 0, fmt.Errorf(
			"GetTermAtIndex(): li=%v > iole=%v", li, len(iml.entries),
		)
	}
	return iml.entries[li-1].TermNo, nil
}

func (iml *InMemoryLog) GetEntriesAfterIndex(afterLogIndex LogIndex) ([]LogEntry, error) {
	iml.lock.Lock()
	defer iml.lock.Unlock()

	if afterLogIndex < iml.lastCompacted {
		return nil, ErrIndexCompacted
	}

	iole := iml.indexOfLastEntry.UnsafeGet()

	if afterLogIndex > iole {
		return nil, fmt.Errorf(
			"afterLogIndex=%v is > iole=%v",
			afterLogIndex,
			iole,
		)
	}

	var numEntriesToGet = uint64(iole - afterLogIndex)

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
	iml.lock.Lock()
	defer iml.lock.Unlock()

	if li < iml.lastCompacted {
		return ErrIndexCompacted
	}
	iole := iml.indexOfLastEntry.UnsafeGet()
	if iole < li {
		return fmt.Errorf("InMemoryLog: setEntriesAfterIndex(%d, ...) but iole=%d", li, iole)
	}
	// delete entries after index
	if iole > li {
		iml.entries = iml.entries[:li]
	}
	// append entries
	iml.entries = append(iml.entries, entries...)

	// update iole
	newIole := LogIndex(len(iml.entries))
	return iml.indexOfLastEntry.UnsafeSet(newIole)
}

func (iml *InMemoryLog) DiscardEntriesBeforeIndex(li LogIndex) error {
	iml.lock.Lock()
	defer iml.lock.Unlock()

	if li <= iml.lastCompacted {
		return ErrIndexCompacted
	}
	iml.lastCompacted = li - 1
	// FIXME: actually throw away entries!
	return nil
}

func (iml *InMemoryLog) AppendEntry(logEntry LogEntry) (LogIndex, error) {
	// return fmt.Errorf("InMemoryLog: EEEE: %v", logEntry)

	iml.lock.Lock()
	defer iml.lock.Unlock()

	iml.entries = append(iml.entries, logEntry)

	// update iole
	newIole := LogIndex(len(iml.entries))
	err := iml.indexOfLastEntry.UnsafeSet(newIole)
	if err != nil {
		return 0, err
	}

	return newIole, nil
}
