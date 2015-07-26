package raft

// Server states.
type ServerState uint

const (
	FOLLOWER ServerState = iota
	CANDIDATE
	LEADER
)

// Election term.
// Initialized to 0 on first boot, increases monotonically.
type TermNo uint64
