// Interfaces that users of this package must implement.

package raft

// Persistent state on all servers
type PersistentState interface {
	// Get the latest term server has seen.
	// (initialized to 0, increases monotonically)
	// The ConsensusModule will only ever use call this method from a
	// single goroutine.
	// (However, tests will call from a different goroutine)
	GetCurrentTerm() TermNo

	// Get the canidate id this server has voted for. ("" if none)
	// The ConsensusModule will only ever call this method from it's
	// single goroutine.
	// (However, tests will call from a different goroutine)
	GetVotedFor() ServerId

	// Set the latest term this server has seen and the candidate
	// it has voted for in this term.
	// The ConsensusModule will only ever use call this method from a
	// single goroutine.
	// TODO: should this call error for decreasing/same term
	// TODO: sync/async semantics & persistence error?!
	SetCurrentTermAndVotedFor(currentTerm TermNo, votedFor ServerId)
}

// "RPC" sending layer.
// This is called "RPC" for raft terminology, but we're using an asynchronous
// messaging layer model.
type RpcSender interface {
	// Send the given RPC message to the given server.
	// The sending should not block the caller.
	// The ConsensusModule will only ever call this method from it's
	// single goroutine.
	// TODO: should this call error if unable to *not* block?
	// TODO: should this call error if rpc type is unknown?
	// TODO: should this call error if bad server id?
	SendAsync(toServer ServerId, rpc interface{})
}
