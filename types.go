package raft

import (
	"errors"
	"fmt"
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

// CommandResult is the result of applying a command to the state machine.
type CommandResult interface{}

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

// ServerStateToString returns a string representation of a ServerState value.
func ServerStateToString(serverState ServerState) string {
	switch serverState {
	case FOLLOWER:
		return "FOLLOWER"
	case CANDIDATE:
		return "CANDIDATE"
	case LEADER:
		return "LEADER"
	default:
		return fmt.Sprintf("Unknown ServerState: %v", serverState)
	}
}
