package follower

import (
	. "github.com/divtxt/raft"
)

// Volatile state on followers
type FollowerVolatileState struct {
	leader ServerId
}

func NewFollowerVolatileState(leader ServerId) *FollowerVolatileState {
	return &FollowerVolatileState{leader}
}

func (fvs *FollowerVolatileState) GetLeader() ServerId {
	return fvs.leader
}
