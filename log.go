package raft

// A state machine command.
// The contents of the slice are opaque to the ConsensusModule.
type Command []byte

// An entry in the Raft Log
type LogEntry struct {
	TermNo
	Command
}

// Log entry index. First index is 1.
type LogIndex uint64

// The Raft Log
// An ordered array of `LogEntry`s with first index 1.
//
// All errors should be indicated using panic(). This includes both invalid
// parameters sent by the consensus module and internal errors in the Log.
// Note that such a panic will shutdown the consensus module.
//
type Log interface {
	// An index of 0 indicates no entries present.
	GetIndexOfLastEntry() LogIndex

	// An index of 0 is invalid for this call.
	GetLogEntryAtIndex(LogIndex) LogEntry

	// Get the term of the entry at the given index.
	// Equivalent to getLogEntryAtIndex(...).TermNo but this call allows
	// the Log implementation to not fetch the Command if that's a useful
	// optimization.
	// An index of 0 is invalid for this call.
	GetTermAtIndex(LogIndex) TermNo

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
	// An index of 0 is valid and implies deleting all entries.
	//
	// A zero length slice and nil both indicate no new entries to be added
	// after deleting.
	SetEntriesAfterIndex(LogIndex, []LogEntry)
}

// Helper method
func GetIndexAndTermOfLastEntry(log Log) (LogIndex, TermNo) {
	lastLogIndex := log.GetIndexOfLastEntry()
	var lastLogTerm TermNo = 0
	if lastLogIndex > 0 {
		lastLogTerm = log.GetTermAtIndex(lastLogIndex)
	}
	return lastLogIndex, lastLogTerm
}
