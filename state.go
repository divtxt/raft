package raft

// Volatile state on all servers
type volatileState struct {
	// index of highest log entry known to be committed
	// (initialized to 0, increases monotonically)
	commitIndex LogIndex

	// #Errata-X1: lastApplied should be as durable as the state machine
	// lastApplied LogIndex
}
