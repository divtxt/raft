package raft

import (
	"testing"
)

// #5.2-p1s2: When servers start up, they begin as followers
func TestCMStartsAsFollower(t *testing.T) {
	ps := newIMPSWithCurrentTerm(TEST_CURRENT_TERM)
	th := new(mockTimeoutHelper)
	cm := newConsensusModuleImpl(ps, nil, th)

	if cm == nil {
		t.Fatal()
	}
	if cm.GetServerState() != FOLLOWER {
		t.Fatal()
	}
}

//
type mockTimeoutHelper struct {
	resetElectionTimeoutCalled bool
}

func (mth *mockTimeoutHelper) resetElectionTimeout() {
	mth.resetElectionTimeoutCalled = true
}

// #5.2-p1s5: If a follower receives no communication over a period of time
// called the election timeout, then it assumes there is no viable leader
// and begins an election to choose a new leader.
// #5.2-p2s1: To begin an election, a follower increments its current term
// and transitions to candidate state.
func TestCMElectionTimeout(t *testing.T) {
	ps := newIMPSWithCurrentTerm(TEST_CURRENT_TERM)
	th := new(mockTimeoutHelper)
	cm := newConsensusModuleImpl(ps, nil, th)

	if cm == nil {
		t.Fatal()
	}
	if cm.GetServerState() != FOLLOWER {
		t.Fatal()
	}

	// Test that the election timeout callback is reset on creation
	if !th.resetElectionTimeoutCalled {
		t.Fatal()
	}

	// Test that election timeout causes a new election
	th.resetElectionTimeoutCalled = false
	cm.ElectionTimeout()
	if ps.GetCurrentTerm() != TEST_CURRENT_TERM+1 {
		t.Fatal()
	}
	if cm.GetServerState() != CANDIDATE {
		t.Fatal()
	}
}
