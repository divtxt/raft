// Interfaces that users of this package must implement.
//
// Notes for implementers:
//
// - Concurrency: the consensus module will only ever call the methods from it's
// single goroutine.
//
// - Errors: all errors should be indicated using panic(). This includes both
// invalid parameters sent by the consensus module and internal errors in the
// implementation. Note that such a panic will shutdown the consensus module.
//

package raft

// Persistent state on all servers.
//
// This state should be persisted to stable storage.
//
// No one else should be modifying these values, and the consensus module does
// not cache these values, so it is recommended that implementations cache the
// values for getter performance.
type PersistentState interface {
	// Get the latest term server has seen.
	// (initialized to 0, increases monotonically)
	GetCurrentTerm() TermNo

	// Get the candidate id this server has voted for. ("" if none)
	GetVotedFor() ServerId

	// Set the latest term this server has seen and the candidate
	// it has voted for in this term.
	// This call should be synchronous i.e. not return until the values
	// have been written to persistent storage.
	//
	// TODO: should this call error for decreasing/same term
	SetCurrentTermAndVotedFor(currentTerm TermNo, votedFor ServerId)
}

// Asynchronous RPC service.
//
// Since the consensus module is implemented as a single threaded component, an
// asynchronous programming model is used for RPC.
// In this model, RPC requests and replies are independent message types that
// are sent and received using the same interfaces.
//
// The choice of RPC protocol is unspecified here.
//
// See rpctypes.go for the various RPC message types.
//
// See the consensus module's ProcessRpcAsync() for incoming RPC.
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
	// reply to the consensus module's goroutine and returns immediately.
	//
	// - An unknown or unexpected rpc reply message will cause the consensus
	// module's goroutine to panic and stop.
	//
	// - If the RPC fails, there is no need to do anything.
	//
	// - It is expected that multiple RPC messages will be sent independently to
	// different servers.
	//
	// - The consensus module only expects to send one RPC to a given server.
	// Since RPC failure is not reported to consensus module, implementations can
	// choose how to handle extra RPCs to a server for which they already have an
	// RPC in flight: cancel the first message, drop the second or enqueue it.
	//
	// - The RPC is time-sensitive and expected to be immediate. If any queueing
	// or retrying is implemented, it should be very limited in time and queue
	// size.
	SendAsync(toServer ServerId, rpc interface{}, replyAsync func(interface{}))
}
