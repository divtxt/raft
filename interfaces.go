// Interfaces that users of this package must implement.

package raft

// Persistent state on all servers
type PersistentState interface {
	// latest term server has seen
	GetCurrentTerm() TermNo
	SetCurrentTerm(currentTerm TermNo)
}
