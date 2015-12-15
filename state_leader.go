package raft

// Volatile state on leaders
type leaderVolatileState struct {
}

// New instance set up for a fresh leader
func newLeaderVolatileState(clusterInfo *ClusterInfo) *leaderVolatileState {
	return &leaderVolatileState{}
}
