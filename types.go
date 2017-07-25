package raft

import (
	"errors"
)

// Raft server states.
type ServerState uint32

const (
	FOLLOWER ServerState = iota
	CANDIDATE
	LEADER
)

var ErrStopped = errors.New("ConsensusModule is stopped")

var ErrNotLeader = errors.New("Not currently in LEADER state")

// Raft election term.
// Initialized to 0 on first boot, increases monotonically.
type TermNo uint64

// A state machine command (in serialized form).
// The contents of the byte slice are opaque to the ConsensusModule.
type Command []byte

// CommitSignal is a channel used to signal when a command is committed.
//
// If and when the command is committed, a value is sent on this channel.
//
// If the ConsensusModule loses leadership, this channel is closed.
// Note that the command may still commit, but the ConsensusModule stops tracking it.
//
type CommitSignal <-chan struct{}

// An entry in the Raft Log
type LogEntry struct {
	TermNo
	Command
}

// Log entry index. First index is 1.
type LogIndex uint64

// An integer that uniquely identifies a server in a Raft cluster.
//
// Zero should not be used as a server id.
//
// See config.ClusterInfo for how this is used in this package.
// The number value does not have a meaning to this package.
// This package also does not know about the network details - e.g. protocol/host/port -
// since the RPC is not part of the package but is delegated to the user.
type ServerId uint64
