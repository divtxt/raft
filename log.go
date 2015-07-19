package raft

// A state machine command.
// The Raft consensus module does not care about an actual type of a Command.
// It is up to you to make sure that the other components that the consensus
// module talks to agree on the actual type.
type Command interface{}

// An entry in the Raft Log
type LogEntry struct {
	TermNo
	Command
}

// Log entry index. First index is 1.
type LogIndex uint64

// The Raft Log
// An ordered array of `LogEntry`s with first index 1.
type Log interface {
	// An index of 0 indicates no entries present.
	getIndexOfLastEntry() LogIndex
	getTermAtIndex(LogIndex) TermNo
	deleteFromIndexToEnd(LogIndex)
}
