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

	// Append a new entry with the given term and given serialized command.
	//
	// This method will only be called when this ConsensusModule is the leader.
	//
	// The raw command will have been approved and serialized by the
	// StateMachine.ReviewAppendCommand() method.
	AppendEntry(termNo TermNo, command Command) error
}

// Read-only subset of the Raft Log.
//
// This will typically be the interface used by a StateMachine implementation.
//
// See Log interface for method descriptions
//
type LogReadOnly interface {
	GetIndexOfLastEntry() (LogIndex, error)
	GetTermAtIndex(LogIndex) (TermNo, error)
	GetEntriesAfterIndex(LogIndex) ([]LogEntry, error)
}

// The State Machine being implemented.
//
// You must implement this interface!
//
// Your service must expose the command part of the state machine through this interface to the
// ConsensusModule. The state machine's query functionality is not exposed here.
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
// commits. To achieve this we have this interface also be a listener for the
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
//
// Concurrency: the ConsensusModule will only ever call the methods of this interface from it's
// single goroutine.
//
// Errors: all errors should be indicated in the return value, and any such error returned by this
// interface will shutdown the ConsensusModule.
//
type StateMachine interface {

	// Review a command and serialize it for entry in the Log.
	//
	// This method will only be called when this ConsensusModule is the leader.
	//
	// The command is considered to be in unserialized form, and this method is responsible
	// for reviewing it, and serializing it if approved.
	//
	// This method can reject the given command if it cannot be applied given the current
	// state of the StateMachine and Log, or if the command itself is bad in some way.
	//
	// The StateMachine is responsible for ensuring that the command cannot later fail when
	// the commitIndex advances to the command's log index. This means that the StateMachine
	// can either look ahead into uncommitted commands to verify this, or it can structure
	// commands in a such a way that the "applying" the command always succeeds, even if the
	// operation intended by the command may not.
	//
	// To indicate an approval, the command should return (<command>, <reply>, nil).
	// If the command is approved, the ConsensusModule will call Log.AppendEntry()
	//
	// To indicate a rejection, the command should return (nil, <reply>, nil). In this case,
	// the entry is not appended to the Log.
	//
	// In both cases, the <reply> value is returned to the originating
	// ConsensusModule.AppendCommandAsync() call.
	//
	ReviewAppendCommand(rawCommand interface{}) (Command, interface{}, error)

	// Notify the state machine that the commitIndex has changed to the given value.
	//
	// commitIndex is the index of highest log entry known to be committed
	// (initialized to 0, increases monotonically)
	//
	// The implementer can apply entries up to this index from the Log to the state machine.
	// (see #RFS-A1 mentioned above)
	//
	// This method should return immediately without blocking. This means that applying log entries
	// to the state machine up to the new commitIndex should be asynchronous to this method call.
	//
	// On startup, the StateMachine can consider the previous commitIndex to be either 0 or to the
	// persisted value of lastApplied. If the state machine is not persisted, there is no difference
	// as lastApplied will also start at 0.
	//
	// It is an error if the value of commitIndex decreases. Note that the upstream commitIndex may
	// NOT be persisted, and may reset to 0 on restart. However, the upstream should never send this
	// initial 0 value to StateMachine, and should always jump to a non-decreasing value.
	//
	// It is an error if the value is beyond the end of the log.
	// (i.e. the given index is greater than indexOfLastEntry)
	//
	CommitIndexChanged(LogIndex) error
}

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
	//
	// This method can reject the given command if it cannot be applied given the current
	// state of the Log and state machine, or if the command itself is bad in some way.
	//
	// The method should return a non-nil value to indicate the result.
	// A nil result is considered an error and will shutdown the ConsensusModule.
	// All other values are opaque to ConsensusModule and will be returned as the result
	// of AppendCommandAsync.
	AppendEntry(termNo TermNo, rawCommand interface{}) (interface{}, error)

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
