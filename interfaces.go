// Interfaces that users of this package must implement.

package raft

// The Raft Log and the service's state machine.
//
// You must implement this interface!
//
// Your service must implement the storage of the Raft log and expose it to the
// ConsensusModule through this interface. Parts of the service's state machine
// also need to be exposed in this interface.
//
// The log is an ordered array of 'LogEntry's with first index 1.
// A LogEntry is a tuple of a Raft term number and a command to be applied
// to the state machine.
//
// Raft describes two state parameters - commitIndex and lastApplied -
// that are used to track which log entries are committed to the log and the
// state machine respectively. However, Raft is focussed on the log and cares
// very little about lastApplied, other than to drive state machine commits.
// Specifically:
//
// #RFS-A1: If commitIndex > lastApplied: increment lastApplied, apply
// log[lastApplied] to state machine (#5.3)
//
// We take advantage of this by having the implementer of this interface own
// the lastApplied value and the above responsibility of driving state machine
// commits. To achieve this we have this interface also be a listener for the
// value of commitIndex.
//
// With this tweak, the implementation of lastApplied is no longer a concern
// of the ConsensusModule. Instead, lastApplied is delegated to the state
// machine via this interface. We capture it's details here:
//
// - lastIndex is the index of highest log entry applied to state machine
// (initialized to 0, increases monotonically)
//
// - (#Errata-X1:) lastApplied should be as durable as the state machine
// From https://github.com/ongardie/dissertation#updates-and-errata :
// Although lastApplied is listed as volatile state, it should be as
// volatile as the state machine. If the state machine is volatile,
// lastApplied should be volatile. If the state machine is persistent,
// lastApplied should be just as persistent.
//
// The ConsensusModule needs all method calls to succeed and any error will
// shutdown the consensus module.
//
type LogAndStateMachine interface {
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
	// We make the Log responsible for choosing how many entries to send. The
	// returned entries will be sent as is in an AppendEntries RPC to a follower.
	// The Log can implement any policy - the simplest: "return just one entry".
	//
	// It is an error if the given index is beyond the end of the log.
	// (i.e. the given index is greater than indexOfLastEntry)
	//
	// If there are entries after the given index, the call must return at
	// least one entry.
	//
	// An index of 0 is invalid for this call.
	// There should be no entries for the Log of a new server.
	GetEntriesAfterIndex(LogIndex) ([]LogEntry, error)

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

	// Append a new entry with the given term and given raw command.
	//
	// This method will only be called when this ConsensusModule is the leader.
	//
	// The command is considered to be in unserialized form, and this method is responsible
	// for serializing it.
	// The command is opaque to the ConsensusModule, and is as sent to AppendCommandAsync().
	AppendEntry(termNo TermNo, rawCommand interface{}) error

	// Notify the Log that the commitIndex has changed to the given value.
	//
	// commitIndex is the index of highest log entry known to be committed
	// (initialized to 0, increases monotonically)
	//
	// The implementer can apply entries up to this index in the state machine.
	// (see #RFS-A1 mentioned above)
	//
	// This method should return immediately without blocking. This means
	// that applying entries to the state machine should be asynchronous /
	// concurrent to this function.
	//
	// It is an error if the value of commitIndex decreases.
	//
	// It is an error if the value is beyond the end of the log.
	// (i.e. the given index is greater than indexOfLastEntry)
	//
	// On startup, the initial value of commitIndex should be set either
	// to 0 or - optionally, and if the state machine is persisted - to the
	// persisted value of lastApplied.
	// (If the state machine is not persisted, the second choice is irrelevant
	// as lastApplied will also be starting at 0)
	CommitIndexChanged(LogIndex) error
}

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

// Asynchronous RPC service.
//
// You must implement this interface!
//
// The choice of RPC protocol is unspecified here.
//
// See Rpc* types (in rpctypes.go) for the various RPC message and reply types.
//
// See ConsensusModule's ProcessRpc...Async methods for incoming RPC.
//
// Notes for implementers:
//
// - The methods should return immediately.
//
// - No guarantee of RPC success is expected.
//
// - A bad server id or unknown rpc type should be treated as an error.
//
// - If the RPC succeeds, the reply rpc should be sent to the replyAsync()
// function parameter.
//
// - replyAsync() will process the reply asynchronously. It sends the rpc
// reply to the ConsensusModule's goroutine and returns immediately.
//
// - If the RPC fails, there is no need to do anything.
//
// - The ConsensusModule only expects to send one RPC to a given server.
// Since RPC failure is not reported to ConsensusModule, implementations can
// choose how to handle extra RPCs to a server for which they already have an
// RPC in flight i.e. cancel the first message and/or drop the second.
//
// - It is expected that multiple RPC messages will be sent independently to
// different servers.
//
// - The RPC is time-sensitive and expected to be immediate. If any queueing
// or retrying is implemented, it should be very limited in time and queue
// size.
type RpcService interface {
	// Send the given RpcAppendEntries message to the given server
	// asynchronously.
	SendRpcAppendEntriesAsync(
		toServer ServerId,
		rpc *RpcAppendEntries,
		replyAsync func(*RpcAppendEntriesReply),
	) error

	// Send the given RpcRequestVote message to the given server
	// asynchronously.
	SendRpcRequestVoteAsync(
		toServer ServerId,
		rpc *RpcRequestVote,
		replyAsync func(*RpcRequestVoteReply),
	) error
}
