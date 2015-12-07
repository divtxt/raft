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

	// TODO: return values for invalid params or log errors
	getLogEntryAtIndex(LogIndex) LogEntry

	// Get the term of the entry at the given index.
	// Equivalent to getLogEntryAtIndex(...).TermNo but this call allows
	// the Log implementation to not fetch the Command if that's a useful
	// optimization.
	// TODO: return values for invalid params or log errors
	// TODO: 0 for 0 and simplify callers and tests
	// TODO: single call for tuple?
	getTermAtIndex(LogIndex) TermNo

	// Set the entries after the given index.
	//
	// Theoretically, the Log can just delete all existing entries
	// following the given index and then append the given new
	// entries after that index.
	//
	// However, Raft properties means that the Log can use this logic:
	// - (AppendEntries receiver step 3.) If an existing entry conflicts with
	// a new one (same index but different terms), delete the existing entry
	// and all that follow it (#5.3)
	// - (AppendEntries receiver step 4.) Append any new entries not already
	// in the log
	//
	// I.e. the Log can choose to set only the entries starting from
	// the first index where the terms of the existing entry and the new
	// entry don't match.
	//
	// Note that an index of 0 is valid and implies deleting all entries.
	// A zero length slice and nil both indicate no new entries to be added
	// after deleting.
	//
	// This method is expected to always succeed. All errors should be indicated
	// by panicking, and this will shutdown the consensus module. This includes
	// both invalid parameters from the caller and internal errors in the Log.
	setEntriesAfterIndex(LogIndex, []LogEntry)
}
