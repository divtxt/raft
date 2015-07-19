package raft

// Election term.
// Initialized to 0 on first boot, increases monotonically.
type TermNo uint64
