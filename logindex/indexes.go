package logindex

import (
	"sync"
)

// Indexes acts as a central hub for the various Raft log indexes.
//
// It allows various components to share the values that some own and others need access to while
// avoiding direct dependencies on each other. It also checks invariants of these values relative
// to each other when any value is updated.
//
// The values are:
// - lastCompacted - owned by the Log
// - lastApplied - owned by the StateMachine
// - commitIndex - owned by the Raft ConsensusModule
// - indexOfLastEntry - owned by the Log
//
// The invariants enforced are:
//
// - lastCompacted <= lastApplied <= commitIndex <= indexOfLastEntry
// - commitIndex cannot decrease
//
type RaftIndexes struct {
	l *sync.Mutex
	//indexOfLastEntry LogIndex
	CommitIndex *WatchedIndex
	//lastApplied      LogIndex
	//lastCompacted    LogIndex
}

// NewRaftIndexes creates a new Indexes instance.
func NewRaftIndexes(
	commitIndexListener ChangeListener,
) *RaftIndexes {
	l := &sync.Mutex{}
	i := &RaftIndexes{
		l,
		NewWatchedIndex(l, commitIndexListener),
	}
	return i
}
