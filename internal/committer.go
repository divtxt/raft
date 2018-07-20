package internal

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
	// When the command at the given log index is later committed and applied to the state machine
	// by this Committer, the value returned by the state machine must be sent on this channel.
	//
	// A call to RemoveListenersAfterIndex that affects this index must close this channel instead.
	//
	// The log index must be greater than the current highestRegisteredIndex and becomes the new
	// value of highestRegisteredIndex. This means that the log index must strictly increase,
	// except when a call to RemoveListenersAfterIndex rewinds the value of highestRegisteredIndex.
	// This also means that no more than one listener can be registered for a given log index,
	// except when an existing listener is closed by RemoveListenersAfterIndex.
	//
	// The log index must be greater than commitIndex.
	//
	// The log index must be less than or equal to the log's indexOfLastEntry.
	//
	// The intent of this method is that an entry at the given index is expected to be appended to
	// the log before a listener is registered for that index.
	//
	// On startup, the Committer can consider the initial value of highestRegisteredIndex to be
	// either 0 or the log's indexOfLastEntry.
	//
	// Note that a listener may not be registered for every log index.
	//
	RegisterListener(logIndex raft.LogIndex) (<-chan raft.CommandResult, error)

	// Remove and close existing listeners for all log indexes after the given log index.
	//
	// If the given index is less than highestRegisteredIndex, then highestRegisteredIndex must
	// be set to the given index.
	//
	// This means that we must be able to register listeners after the given index, even if
	// listeners were previously registered for such indexes.
	//
	// The log index must be greater than or equal to the latest commitIndex.
	//
	// The intent of this method is to handle the case where a ConsensusModule loses leadership,
	// and the new leader replaces the log entries after the given log index.
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
	CommitAsync(commitIndex raft.LogIndex) error
}
