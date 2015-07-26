package raft

// Persistent state on all servers
type PersistentState struct {
	currentTerm TermNo
}

// Volatile state on all servers
type VolatileState struct {
	// index of highest log entry known to be committed
	// (initialized to 0, increases monotonically)
	commitIndex LogIndex

	// index of highest log entry applied to state machine
	// (initialized to 0, increases monotonically)
	lastApplied LogIndex
}
