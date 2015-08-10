package raft

import (
	"testing"
	"time"
)

// #5.2-p1s2: When servers start up, they begin as followers
func TestCMStartsAsFollower(t *testing.T) {
	ps := newIMPSWithCurrentTerm(TEST_CURRENT_TERM)
	ts := TimeSettings{5 * time.Millisecond, 50 * time.Millisecond}
	cm := NewConsensusModule(ps, nil, ts)
	defer cm.StopAsync()

	if cm == nil {
		t.Fatal()
	}
	if cm.GetServerState() != FOLLOWER {
		t.Fatal()
	}
}

func TestCMStop(t *testing.T) {
	ps := newIMPSWithCurrentTerm(TEST_CURRENT_TERM)
	ts := TimeSettings{5 * time.Millisecond, 50 * time.Millisecond}
	cm := NewConsensusModule(ps, nil, ts)

	if cm.IsStopped() {
		t.Error()
	}

	cm.StopAsync()
	time.Sleep(1 * time.Millisecond)

	if !cm.IsStopped() {
		t.Error()
	}
}

// #5.2-p1s5: If a follower receives no communication over a period of time
// called the election timeout, then it assumes there is no viable leader
// and begins an election to choose a new leader.
// #5.2-p2s1: To begin an election, a follower increments its current term
// and transitions to candidate state.
func TestCMElectionTimeout(t *testing.T) {
	ps := newIMPSWithCurrentTerm(TEST_CURRENT_TERM)
	ts := TimeSettings{5 * time.Millisecond, 50 * time.Millisecond}
	cm := NewConsensusModule(ps, nil, ts)
	defer cm.StopAsync()

	if cm == nil {
		t.Fatal()
	}
	if cm.GetServerState() != FOLLOWER {
		t.Fatal()
	}

	// Test that a tick before election timeout causes no state change.
	time.Sleep(1 * time.Millisecond)
	// cm.pause()
	if ps.GetCurrentTerm() != TEST_CURRENT_TERM {
		t.Fatal()
	}
	if cm.GetServerState() != FOLLOWER {
		t.Fatal()
	}

	// Test that election timeout causes a new election
	time.Sleep(50 * time.Millisecond)
	// cm.pause()
	if ps.GetCurrentTerm() != TEST_CURRENT_TERM+1 {
		t.Fatal()
	}
	if cm.GetServerState() != CANDIDATE {
		t.Fatal()
	}
}
