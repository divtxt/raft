package raft

// Raft server states.
type ServerState uint32

const (
	FOLLOWER ServerState = iota
	CANDIDATE
	LEADER
)

// Raft election term.
// Initialized to 0 on first boot, increases monotonically.
type TermNo uint64

// A string identifying a server in a Raft cluster.
//
// The contents of the string is opaque to this package.
// A blank string should not be used as a server id.
//
// In practice, since this is used here for RPC, a network/service specifier
// is probably the most useful e.g. "<host>:<port>".
type ServerId string
