package consensus

import (
	"github.com/divtxt/raft"
)

// ICommitter is the internal interface that PassiveConsensusModule uses
// to delegate the work of applying committed log entries to the state machine
// and notifying clients that are waiting for those entries to be committed.
//
// Specifically, the following responsibility is being delegated:
//
// #RFS-A1: If commitIndex > lastApplied: increment lastApplied, apply
// log[lastApplied] to state machine (#5.3)
//
// Concurrency: PassiveConsensusModule will only ever make one call to this interface at a time.
//
type ICommitter interface {

	// Register the listener for a future commit at the given log index.
	//
	// After a command at the given log index is committed and applied to the state machine,
	// the value returned by the state machine should be sent on this channel.
	//
	// The channel should be closed instead if affected by RemoveListenersAfterIndex.
	//
	// The log index to this method will generally strictly increase, except that a call to
	// ClearListenersAfterIndex rewinds the expected minimum for the next log index.
	//
	// The log index should always be more than the latest commitIndex.
	//
	// No more than one listener should be registered for a given log index, except in the case
	// where the existing listener is closed by ClearListenersAfterIndex.
	//
	// Note that a listener may not be registered for every log index.
	//
	RegisterListener(logIndex raft.LogIndex) <-chan raft.CommandResult

	// Remove existing listeners for all log indexes after the given log index.
	//
	// The channels for those listeners should be closed.
	//
	// This method should remove all listeners before returning.
	//
	// The given index should always be greater than or equal to the latest commitIndex.
	//
	// This situation occurs due to the ConsensusModule loses leadership, and the new leader
	// replacing the log entries after the given log index.
	//
	RemoveListenersAfterIndex(afterIndex raft.LogIndex)

	// Commit log entries to the state machine asynchronously up to the given index.
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
	// On startup, the Committer can consider the previous commitIndex to be either
	// 0 or the persisted value of lastApplied. If the state machine is not persisted, there is no
	// difference as lastApplied will also start at 0.
	//
	// It is an error if the value of commitIndex decreases. Note that the upstream commitIndex may
	// NOT be persisted, and may reset to 0 on restart. However, the upstream should never send this
	// initial 0 value to StateMachine, and should never send a value that is less than a persisted
	// value of lastApplied.
	//
	// It is an error if the value is beyond the end of the log.
	// (i.e. the given index is greater than indexOfLastEntry)
	CommitAsync(commitIndex raft.LogIndex)
}
