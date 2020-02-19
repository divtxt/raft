package follower_test

import (
	"testing"

	"github.com/divtxt/raft/consensus/follower"
)

func TestFollowerVolatileState(t *testing.T) {
	// Known leader
	fvs := follower.NewFollowerVolatileState(101)
	if fvs.GetLeader() != 101 {
		t.Fatal(fvs)
	}

	// Unknown leader
	fvs = follower.NewFollowerVolatileState(0)
	if fvs.GetLeader() != 0 {
		t.Fatal(fvs)
	}
}
