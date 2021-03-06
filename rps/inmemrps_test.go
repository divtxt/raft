package rps_test

import (
	"github.com/divtxt/raft/rps"
	"github.com/divtxt/raft/testhelpers"
	"testing"
)

// Run the blackbox test on InMemoryRaftPersistentState
func TestInMemoryRaftPersistentState(t *testing.T) {
	imps := rps.NewIMPSWithCurrentTerm(0)
	testhelpers.BlackboxTest_RaftPersistentState(t, imps)
}
