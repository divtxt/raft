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

// FIXME: this needs actual values for debugging
var ErrIndexCompacted = errors.New("Given index is less than or equal to lastCompacted")

// Raft election term.
// Initialized to 0 on first boot, increases monotonically.
type TermNo uint64

// A state machine command (in serialized form).
// The contents of the byte slice are opaque to the ConsensusModule.
type Command []byte

// CommandResult is the result of applying a command to the state machine.
// The contents of the byte slice are opaque to the ConsensusModule.
type CommandResult interface{}

// An entry in the Raft Log
type LogEntry struct {
	TermNo
	Command
}

// Log entry index. First index is 1.
type LogIndex uint64

type IndexChangeListener func(new LogIndex)

// A WatchableIndex is a LogIndex that notifies listeners when the value changes.
//
// When the underlying LogIndex value is changed, all registered listeners are
// called in registered order.
// The listener is guaranteed that another change will not occur until it
// has returned to this method.
//
// This is implemented by logindex.WatchedIndex.
type WatchableIndex interface {
	// Get the current value.
	Get() LogIndex

	// Add the given callback as a listener for changes.
	// This is NOT safe to call from a listener.
	AddListener(didChangeListener IndexChangeListener)
}

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
