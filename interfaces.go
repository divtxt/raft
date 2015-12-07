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

// Persistent state on all servers
type PersistentState interface {
	// Get the latest term server has seen.
	// (initialized to 0, increases monotonically)
	// The ConsensusModule will only ever use call this method from a
	// single goroutine.
	GetCurrentTerm() TermNo

	// Get the candidate id this server has voted for. ("" if none)
	// The ConsensusModule will only ever call this method from it's
	// single goroutine.
	GetVotedFor() ServerId

	// Set the latest term this server has seen and the candidate
	// it has voted for in this term.
	// The ConsensusModule will only ever use call this method from a
	// single goroutine.
	// TODO: should this call error for decreasing/same term
	// TODO: sync/async semantics & persistence error?!
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
type RpcSender interface {
	// Send the given RPC message to the given server asynchronously.
	//
	// Notes for implementers:
	//
	// - No guarantee of success is expected.
	//
	// - When the RPC completes, the result should be sent via the consensus
	// module's ProcessRpcAsync() function.
	//
	// - If the RPC fails, there is no need to notify the consensus module.
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
	//
	// TODO: should this call error if rpc type is unknown?
	// TODO: should this call error if bad server id?
	SendAsync(toServer ServerId, rpc interface{})
}
