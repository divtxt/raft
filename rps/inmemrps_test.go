package rps_test

import (
	"github.com/divtxt/raft/rps"
	"testing"
)

// Run the blackbox test on InMemoryRaftPersistentState
func TestInMemoryRaftPersistentState(t *testing.T) {
	imps := rps.NewIMPSWithCurrentTerm(0)
	rps.BlackboxTest_RaftPersistentState(t, imps)
}
