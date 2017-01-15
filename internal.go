// Internal interfaces.

package raft

// LogReadOnly is a read-only subset of the Log interface for internal use.
type LogReadOnly interface {
	GetIndexOfLastEntry() (LogIndex, error)
	GetTermAtIndex(LogIndex) (TermNo, error)
	GetEntriesAfterIndex(LogIndex, uint64) ([]LogEntry, error)
}

// ChangeListener is an internal interface to the component that applies committed entries
// from the raft log to the state machine.
//
// Specifically, the following responsibility is delegated to the implementation of this interface:
//
// #RFS-A1: If commitIndex > lastApplied: increment lastApplied, apply
// log[lastApplied] to state machine (#5.3)
//
// Concurrency: the consensus module will only ever make one call this interface at a time.
//
type ChangeListener interface {

	// CommitIndexChanged lets this interface know that commitIndex has changed to the given value.
	//
	// commitIndex is the index of highest log entry known to be committed
	// (initialized to 0, increases monotonically)
	//
	// This means that entries up to this index from the Log can be committed to the state machine.
	// (see #RFS-A1 mentioned above)
	//
	// This method should return immediately without blocking. This means that applying log entries
	// to the state machine up to the new commitIndex should be asynchronous to this method call.
	//
	// On startup, the ChangeListener can consider the previous commitIndex to be either 0 or
	// the persisted value of lastApplied. If the state machine is not persisted, there is no
	// difference as lastApplied will also start at 0.
	//
	// It is an error if the value of commitIndex decreases. Note that the upstream commitIndex may
	// NOT be persisted, and may reset to 0 on restart. However, the upstream should never send this
	// initial 0 value to StateMachine, and should never send a value that is less than a persisted
	// value of lastApplied.
	//
	// It is an error if the value is beyond the end of the log.
	// (i.e. the given index is greater than indexOfLastEntry)
	CommitIndexChanged(LogIndex)
}
