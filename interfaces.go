// Interfaces that users of this package must implement.

package raft

// Persistent state on all servers
type PersistentState interface {
	// latest term server has seen
	// (initialized to 0, increases monotonically)
	GetCurrentTerm() TermNo

	// TODO: should this call error for decreasing/same term
	SetCurrentTerm(currentTerm TermNo)
}
