package internal

import (
	. "github.com/divtxt/raft"
)

// LogReadOnly is a read-only subset of the Log interface for internal use.
type LogReadOnly interface {
	GetLastCompacted() (LogIndex, error)
	GetIndexOfLastEntry() (LogIndex, error)
	GetTermAtIndex(LogIndex) (TermNo, error)
	GetEntriesAfterIndex(LogIndex) ([]LogEntry, error)
}

// LogTailOnly is the subset of the Log interface used by components that will only access the
// non-compacted tail of the raft log.
//
// FIXME: is this correct?! need to check all components & calls, and edge cases/values!
// By this we mean that they will never call Log methods with an index that is less than the value
// of lastApplied. This means that we don't need to worry about log compaction, don't expect calls
// to GetLastCompacted(), and should never be returned an ErrIndexCompacted error.
//
type LogTailOnly interface {
	GetIndexOfLastEntry() (LogIndex, error)
	GetTermAtIndex(LogIndex) (TermNo, error)
	GetEntriesAfterIndex(LogIndex) ([]LogEntry, error)
	SetEntriesAfterIndex(LogIndex, []LogEntry) error
	AppendEntry(LogEntry) (LogIndex, error)
}

// LogTailOnlyRO is the read-only subset of LogTailOnly
type LogTailOnlyRO interface {
	GetIndexOfLastEntry() (LogIndex, error)
	GetTermAtIndex(LogIndex) (TermNo, error)
	GetEntriesAfterIndex(LogIndex) ([]LogEntry, error)
}

// LogTailOnlyWO is the write-only subset of LogTailOnly
type LogTailOnlyWO interface {
	SetEntriesAfterIndex(LogIndex, []LogEntry) error
	AppendEntry(LogEntry) (LogIndex, error)
}
