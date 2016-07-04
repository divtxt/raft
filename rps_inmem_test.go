package raft

import (
	"testing"
)

// Run the blackbox test on InMemoryRaftPersistentState
func TestInMemoryRaftPersistentState(t *testing.T) {
	imps := NewIMPSWithCurrentTerm(0)
	BlackboxTest_RaftPersistentState(t, imps)
}
