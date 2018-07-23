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
}

func NewInMemoryLog() (*InMemoryLog, error) {
	entries := []LogEntry{}
	iml := &InMemoryLog{
		1,
		entries,
	}
	return iml, nil
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

func (iml *InMemoryLog) GetEntryAtIndex(li LogIndex) (LogEntry, error) {
	if li < iml.indexOfFirstEntry {
		return LogEntry{}, ErrIndexBeforeFirstEntry
	}

	iole, err := iml.GetIndexOfLastEntry()
	if err != nil {
		return LogEntry{}, err
	}

	if iole < li {
		return LogEntry{}, ErrIndexAfterLastEntry
		//fmt.Errorf(
		//	"li=%v is > iole=%v",
		//	li,
		//	iole,
		//)
	}

	entry := iml.entries[li-1]

	return entry, nil
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
