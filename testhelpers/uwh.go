package testhelpers

import (
	"github.com/divtxt/raft/logindex"
)

// WatchedIndex that has no locking. Meant only for tests.
func NewUnlockedWatchedIndex() *logindex.WatchedIndex {
	return logindex.NewWatchedIndex(&noOpLocker{}, nil)
}

type noOpLocker struct{}

func (l *noOpLocker) Lock()   {}
func (l *noOpLocker) Unlock() {}
