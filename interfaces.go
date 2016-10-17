// Interfaces that users of this package must implement.

package raft

// The Raft Log.
//
// You must implement this interface!
//
// The log is an ordered array of 'LogEntry's with first index 1.
// A LogEntry is a tuple of a Raft term number and a command to be applied
// to the state machine.
//
// If you are writing your own implementation, please understand the concurrency requirements and
// error handling behavior:
//
// Concurrency:
// All methods should be concurrent safe and a call to one method from other goroutines should
// try to avoid blocking a call to any other method made by the ConsensusModule goroutine.
// In practice, the non-blocking requirement is narrower:
// The ConsensusModule will only ever call the methods of this interface from it's
// single goroutine. Meanwhile, the implementation of the state machine's apply committed command
// action will probably call another method from a different goroutine.
// Since only the ConsensusModule should be calling the SetEntriesAfterIndex() and AppendEntry()
// methods, implementations should not have to deal with concurrent calls to those two methods.
//
// Errors:
// All errors should be indicated in the return value, and any such error returned by this
// interface will shutdown the ConsensusModule.
//
type Log interface {
	// Get the index of the last entry in the log.
	// An index of 0 indicates no entries present.
	// This should be 0 for the Log of a new server.
	GetIndexOfLastEntry() (LogIndex, error)

	// Get the term of the entry at the given index.
	//
	// It is an error if the given index is beyond the end of the log.
	// (i.e. the given index is greater than indexOfLastEntry)
	//
	// An index of 0 is invalid for this call.
	// There should be no entries for the Log of a new server.
	GetTermAtIndex(LogIndex) (TermNo, error)

	// Get multiple entries after the given index.
	//
	// This method should as many entries as available after the given index up to the
	// given count of maxEntries.
	//
	// The returned entries will be sent as is in an AppendEntries RPC to a follower.
	//
	// It is an error if the given index is beyond the end of the log.
	// (i.e. the given index is greater than indexOfLastEntry)
	//
	// If there are entries after the given index, the call must return at
	// least one entry.
	//
	// An index of 0 is invalid for this call.
	// There should be no entries for the Log of a new server.
	GetEntriesAfterIndex(li LogIndex, maxEntries uint64) ([]LogEntry, error)

	// Set the entries after the given index.
	//
	// This method will only be called when this ConsensusModule is a follower.
	//
	// The commands in the given entries are in serialized form.
	//
	// It is an error if commands after the given index have already been
	// applied to the state machine.
	// (i.e. the given index is less than commitIndex)
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
	SetEntriesAfterIndex(LogIndex, []LogEntry) error

	// Append a new entry with the given term and given serialized command.
	//
	// The index of the new entry should be returned.
	// (This should match indexOfLastEntry)
	//
	// This method will only be called when this ConsensusModule is the leader.
	AppendEntry(LogEntry) (LogIndex, error)
}

// Read-only subset of the Raft Log.
//
// This will typically be the interface used by a state machine.
//
// See Log interface for method descriptions
//
type LogReadOnly interface {
	GetIndexOfLastEntry() (LogIndex, error)
	GetTermAtIndex(LogIndex) (TermNo, error)
	GetEntriesAfterIndex(LogIndex, uint64) ([]LogEntry, error)
}

// Changes listener interface.
//
// You must implement this interface!
//
// This interface is meant to be a callback interface that lets the state machine listen for
// changes from the ConsensusModule.
//
// All methods should return immediately without blocking.
//
// Concurrency: the ConsensusModule will only ever call one method of this interface at a time.
//
// Raft describes two state parameters - commitIndex and lastApplied -
// that are used to track which log entries are committed to the log and the
// state machine respectively. However, Raft is focussed on the log and cares
// very little about lastAplied, other than to drive state machine commits.
// Specifically:
//
// #RFS-A1: If commitIndex > lastApplied: increment lastApplied, apply
// log[lastApplied] to state machine (#5.3)
//
// We take advantage of this by having the implementer of this interface own
// the lastApplied value and the above responsibility of driving state machine
// commits. To achieve this we have this interface be a listener for the
// value of commitIndex.
//
// With this tweak, the implementation of lastApplied is no longer a concern
// of the ConsensusModule. Instead, lastApplied is delegated to the state
// machine via this interface. We capture it's details here:
//
// - lastApplied is the index of highest log entry applied to state machine
// (initialized to 0, increases monotonically)
//
// - (#Errata-X1:) lastApplied should be as durable as the state machine
// From https://github.com/ongardie/dissertation#updates-and-errata :
// Although lastApplied is listed as volatile state, it should be as
// volatile as the state machine. If the state machine is volatile,
// lastApplied should be volatile. If the state machine is persistent,
// lastApplied should be just as persistent.
//
type ChangeListener interface {

	// Notify the listener that the commitIndex has changed to the given value.
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
	// On startup, the RaftChangeListener can consider the previous commitIndex to be either 0 or
	// the persisted value of lastApplied. If the state machine is not persisted, there is no
	// difference as lastApplied will also start at 0.
	//
	// It is an error if the value of commitIndex decreases. Note that the upstream commitIndex may
	// NOT be persisted, and may reset to 0 on restart. However, the upstream should never send this
	// initial 0 value to StateMachine, and should always jump to a non-decreasing value.
	//
	// It is an error if the value is beyond the end of the log.
	// (i.e. the given index is greater than indexOfLastEntry)
	CommitIndexChanged(LogIndex)
}

// Raft persistent state on all servers.
//
// You must implement this interface!
//
// This state should be persisted to stable storage.
//
// The initial values should be 0 for the currentTerm and "" for votedFor.
//
// No one else should modify these values, and the ConsensusModule does not
// cache these values, so it is recommended that implementations cache the
// values for getter performance.
type RaftPersistentState interface {
	// Get the latest term server has seen.
	// (initialized to 0, increases monotonically)
	GetCurrentTerm() TermNo

	// Get the candidate id this server has voted for. ("" if none)
	GetVotedFor() ServerId

	// Set the latest term this server has seen.
	//
	// The following should return an error without changing the current
	// values:
	//
	//  - trying to set a value of 0
	//  - trying to set a value less than the current value
	//
	// If the new value is different from the current value, the current votedFor
	// value for this new term should be set to blank.
	//
	// This call should be synchronous i.e. not return until the values
	// have been written to persistent storage.
	SetCurrentTerm(currentTerm TermNo) error

	// Set the voted for value for the current term.
	//
	// The following should return an error without changing the current
	// values:
	//
	//  - trying to set the value when currentTerm is 0
	//  - trying to set a blank value
	//  - trying to set the value when the current value is not blank
	//
	// This call should be synchronous i.e. not return until the values
	// have been written to persistent storage.
	SetVotedFor(votedFor ServerId) error
}

// RPC service.
//
// You must implement this interface!
//
// The choice of RPC protocol is unspecified here.
//
// See Rpc* types (in rpctypes.go) for the various RPC message and reply types.
//
// See ConsensusModule's ProcessRpc* methods for incoming RPC.
//
// Notes for implementers:
//
// - The methods are expected to be synchronous, and may block until the result is available.
// The ConsensusModule will make each call to this interface from a separate goroutine to avoid
// blocking itself.
//
// - Methods can expect server id to be valid & rpc parameter to be non-null. (and panic if not)
//
// - No guarantee of RPC success is expected. Methods should return nil if they decide not to make
// an RPC call, or if there are any I/O errors or timeouts.
//
// - The ConsensusModule will send multiple concurrent RPCs to different servers, and this is
// expected to work.
//
// - The ConsensusModule may also send multiple concurrent RPCs to a specific server, but the
// implementation needs to only make one RPC call at a time to a given server. It is recommended
// that the implementation try to avoid actually making multiple concurrent calls to a given
// server. The implementation can choose how to handle this case e.g. if there is an RPC call in
// flight then return nil any concurrent call to the same server.
//
// - The RPC is time-sensitive and expected to be immediate. If any queueing or retrying is
// implemented, it should be very limited in time and queue size.
type RpcService interface {
	// Send the given RpcAppendEntries message to the given server and get the reply.
	RpcAppendEntries(toServer ServerId, rpc *RpcAppendEntries) *RpcAppendEntriesReply

	// Send the given RpcRequestVote message to the given server and get the reply.
	RpcRequestVote(toServer ServerId, rpc *RpcRequestVote) *RpcRequestVoteReply
}
