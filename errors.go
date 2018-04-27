package raft

import (
	"github.com/go-errors/errors"
)

var errStopped = errors.Errorf("ConsensusModule is stopped")

func NewErrStopped() error {
	return errors.New(errStopped)
}

func IsErrStopped(e error) bool {
	return errors.Is(e, errStopped)
}

var errNotLeader = errors.Errorf("Not currently in LEADER state")

func NewErrNotLeader() error {
	return errors.New(errNotLeader)
}

func IsErrNotLeader(e error) bool {
	return errors.Is(e, errNotLeader)
}
