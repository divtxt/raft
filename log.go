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

// The Raft Log.
//
// This is a combined interface that the ConsensusModule uses to
// manage the Raft log and the state machine.
//
// The log is an ordered array of 'LogEntry's with first index 1.
// A LogEntry is a tuple of a Raft term number and a command to be applied
// to the state machine.
//
// All errors should be indicated using panic(). This includes both invalid
// parameters sent by the consensus module and internal errors in the Log.
// Note that such a panic will shutdown the consensus module.
//
type Log interface {
	// Get the index of the last entry in the log.
	// An index of 0 indicates no entries present.
	// This should be 0 for the Log of a new server.
	GetIndexOfLastEntry() LogIndex

	// Get the LogEntry at the given index.
	// An index of 0 is invalid for this call.
	// There should be no entries for the Log of a new server.
	GetLogEntryAtIndex(LogIndex) LogEntry

	// Get the term of the entry at the given index.
	// Equivalent to getLogEntryAtIndex(...).TermNo but this call allows
	// the Log implementation to not fetch the Command if that's a useful
	// optimization.
	// An index of 0 is invalid for this call.
	GetTermAtIndex(LogIndex) TermNo

	// Set the entries after the given index.
	//
	// It is an error if commands after the given index have already been
	// applied to the state machine.
	// (i.e. given index is less than lastIndexAppliedToStateMachine)
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

	// Get the index of the last entry that has been applied to
	// the state machine.
	//
	// index of highest log entry applied to state machine
	// (initialized to 0, increases monotonically)
	//
	// This should be 0 for the Log of a new server.
	//
	// #Errata-X1: lastApplied should be as durable as the state machine
	// From https://github.com/ongardie/dissertation#updates-and-errata :
	// Although lastApplied is listed as volatile state, it should be as
	// volatile as the state machine. If the state machine is volatile,
	// lastApplied should be volatile. If the state machine is persistent,
	// lastApplied should be just as persistent.
	GetLastApplied() LogIndex

	// Apply the next command to be applied to the state machine.
	//
	// This will result in lastIndexAppliedToStateMachine being incremented.
	//
	// It is an error if there is no next command to be applied i.e.
	// if lastApplied == indexOfLastEntry.
	ApplyNextCommandToStateMachine()
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
