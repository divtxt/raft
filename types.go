package raft

// Server states.
type ServerState uint32

const (
	FOLLOWER ServerState = iota
	CANDIDATE
	LEADER
)

// Election term.
// Initialized to 0 on first boot, increases monotonically.
type TermNo uint64

// Server id - the contents are opaque to this module except for
// a blank string indicating a none value.
type ServerId string
