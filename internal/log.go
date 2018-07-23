package internal

import (
	. "github.com/divtxt/raft"
)

// LogReadOnly is a read-only subset of the Log interface for internal use.
type LogReadOnly interface {
	GetIndexOfFirstEntry() (LogIndex, error)
	GetIndexOfLastEntry() (LogIndex, error)
	GetTermAtIndex(LogIndex) (TermNo, error)
	GetEntryAtIndex(LogIndex) (LogEntry, error)
}

// LogTailOnly is the subset of the Log interface used by components that will only access the
// non-compacted tail of the raft log.
//
// By this we mean that they will never call Log methods with an index that is less than the value
// of lastApplied. This means that we don't need to worry about log compaction, don't expect calls
// to GetIndexOfFirstEntry(), and should never be returned an ErrIndexBeforeFirstEntry error.
//
type LogTailOnly interface {
	GetIndexOfLastEntry() (LogIndex, error)
	GetTermAtIndex(LogIndex) (TermNo, error)
	GetEntryAtIndex(LogIndex) (LogEntry, error)
	SetEntriesAfterIndex(LogIndex, []LogEntry) error
	AppendEntry(LogEntry) (LogIndex, error)
}

// LogTailOnlyRO is the read-only subset of LogTailOnly
type LogTailOnlyRO interface {
	GetIndexOfLastEntry() (LogIndex, error)
	GetTermAtIndex(LogIndex) (TermNo, error)
	GetEntryAtIndex(LogIndex) (LogEntry, error)
}

// LogTailOnlyWO is the write-only subset of LogTailOnly
type LogTailOnlyWO interface {
	SetEntriesAfterIndex(LogIndex, []LogEntry) error
	AppendEntry(LogEntry) (LogIndex, error)
}
