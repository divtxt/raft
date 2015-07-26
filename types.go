package raft

// Server states.
type ServerMode uint

const (
	FOLLOWER ServerMode = iota
	CANDIDATE
	LEADER
)

// Election term.
// Initialized to 0 on first boot, increases monotonically.
type TermNo uint64
