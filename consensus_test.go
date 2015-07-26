package raft

import (
	"testing"
)

func NewConsensusModule() *ConsensusModule {
	return &ConsensusModule{
		// #5.2-p1s2: When servers start up, they begin as followers
		FOLLOWER,
		PersistentState{TEST_CURRENT_TERM, nil},
		VolatileState{},
	}
}

// #5.2-p1s2: When servers start up, they begin as followers
func TestCMStartsAsFollower(t *testing.T) {
	var cm *ConsensusModule
	cm = NewConsensusModule()

	if cm == nil {
		t.Fatal()
	}
	if cm.serverMode != FOLLOWER {
		t.Fatal()
	}
}
