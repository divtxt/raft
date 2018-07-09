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
// All methods should be concurrent safe and a call to one method should
// try to avoid blocking a call to any other method.
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
	// The returned entries will be sent as is in an AppendEntries RPC to a follower.
	//
	// This method is expected to decide how many entries to return based on some policy.
	// This interface itself has no specific requirements on the policy, but it is expected
	// that such a policy will be based on various cluster behavior as well as RPC behavior.
	//
	// If there are entries after the given index, the call must return at least one entry.
	//
	// It is an error if the given index is beyond the end of the log.
	// (i.e. the given index is greater than indexOfLastEntry)
	//
	// An index of 0 is invalid for this call.
	// There should be no entries for the Log of a new server.
	GetEntriesAfterIndex(li LogIndex) ([]LogEntry, error)

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
	// (This should match the new indexOfLastEntry)
	//
	// This method will only be called when this ConsensusModule is the leader.
	AppendEntry(LogEntry) (LogIndex, error)
}

// LogReadOnly is a read-only subset of the Log interface for internal use.
type LogReadOnly interface {
	GetIndexOfLastEntry() (LogIndex, error)
	GetTermAtIndex(LogIndex) (TermNo, error)
	GetEntriesAfterIndex(LogIndex) ([]LogEntry, error)
}

// StateMachine is the interface that the state machine must expose to Raft.
//
// You must implement this interface!
//
// Raft will apply committed commands from the log to the state machine through this interface.
//
// For a given current state and a given command, the new state and returned result
// should be completely deterministic.
//
// Concurrency: this interface will receive only one call at a time. However, note that this
// will be asynchronous to the ConsensusModule. This means that the ConsensusModule can
// commit entries faster than they are applied to the state machine.
//
// Raft describes two state parameters - commitIndex and lastApplied -
// that are used to track which log entries are committed to the log and the
// state machine respectively. However, Raft is focused on the log and cares
// very little about lastApplied, other than to drive state machine commits.
//
// Further, because lastApplied should be persisted with the state machine, we delegate
// lastApplied to the state machine via this interface.
//
// Note that commitIndex may NOT be persisted, and may reset to 0 on restart. However, the
// ConsensusModule should never send this initial 0 value to ApplyCommand(), and should always
// jump to a valid value when calling that method.
//
// Relevant technical details:
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
type StateMachine interface {
	// GetLastApplied should return the value of lastApplied.
	//
	// This is called by the ConsensusModule when it starts and this value is used to avoid
	// replaying entries that have already been applied.
	//
	// For a new state (or non-persistent) machine this value should start at 0.
	GetLastApplied() LogIndex

	// ApplyCommand should apply the given command to the state machine.
	//
	// The log index of the command is given and this should become the new value of lastApplied.
	//
	// This method should return only after all changes have been applied.
	//
	// The given log index should be one more than the current value of lastApplied and this method
	// should panic if this is not the case.
	// FIXME: change the above once raft-related log entries are added
	ApplyCommand(logIndex LogIndex, command Command) CommandResult
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

	// Get the candidate id this server has voted for. (0 if none)
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
	//  - trying to set a server id of 0
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
