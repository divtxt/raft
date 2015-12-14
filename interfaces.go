// Interfaces that users of this package must implement.

package raft

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
type PersistentState interface {
	// Get the latest term server has seen.
	// (initialized to 0, increases monotonically)
	GetCurrentTerm() TermNo

	// Get the candidate id this server has voted for. ("" if none)
	GetVotedFor() ServerId

	// Set the latest term this server has seen.
	//
	// The following are errors and should panic without chaning the current
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
	SetCurrentTerm(currentTerm TermNo)

	// Set the voted for value for the current term.
	//
	// The following are errors and should panic without chaning the current
	// values:
	//
	//  - trying to set the value when currentTerm is 0
	//  - trying to set a blank value
	//  - trying to set the value when the current value is not blank
	//
	// This call should be synchronous i.e. not return until the values
	// have been written to persistent storage.
	SetVotedFor(votedFor ServerId)
}

// Asynchronous RPC service.
//
// You must implement this interface!
//
// The choice of RPC protocol is unspecified here.
//
// See Rpc* types (in rpctypes.go) for the various RPC message and reply types.
//
// See ConsensusModule's ProcessRpcAsync (in raft.go) for incoming RPC.
type RpcService interface {
	// Send the given RPC message to the given server asynchronously.
	//
	// Notes for implementers:
	//
	// - This method should return immediately.
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
	// - An unknown or unexpected rpc reply message will cause the
	// ConsensusModule's goroutine to panic and stop.
	//
	// - If the RPC fails, there is no need to do anything.
	//
	// - It is expected that multiple RPC messages will be sent independently to
	// different servers.
	//
	// - The ConsensusModule only expects to send one RPC to a given server.
	// Since RPC failure is not reported to ConsensusModule, implementations can
	// choose how to handle extra RPCs to a server for which they already have an
	// RPC in flight i.e. cancel the first message and/or drop the second.
	//
	// - The RPC is time-sensitive and expected to be immediate. If any queueing
	// or retrying is implemented, it should be very limited in time and queue
	// size.
	SendAsync(toServer ServerId, rpc interface{}, replyAsync func(interface{}))
}
