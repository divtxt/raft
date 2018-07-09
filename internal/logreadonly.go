package internal

import (
	. "github.com/divtxt/raft"
)

// LogReadOnly is a read-only subset of the Log interface for internal use.
type LogReadOnly interface {
	GetIndexOfLastEntry() (LogIndex, error)
	GetTermAtIndex(LogIndex) (TermNo, error)
	GetEntriesAfterIndex(LogIndex) ([]LogEntry, error)
}
