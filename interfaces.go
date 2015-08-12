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
	// The ConsensusModule will only ever use call this method from a
	// single goroutine.
	// (However, tests will call from a different goroutine)
	GetVotedFor() ServerId

	// Set the latest term this server has seen and the candidate
	// it has voted for in this term.
	// The ConsensusModule will only ever use call this method from a
	// single goroutine.
	// TODO: should this call error for decreasing/same term
	// TODO: persistence error?!
	SetCurrentTermAndVotedFor(currentTerm TermNo, votedFor ServerId)
}
