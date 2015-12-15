package raft

// Volatile state on leaders
// (Reinitialized after election)
type leaderVolatileState struct {
	// for each server, index of the next log entry to send to that server
	// (initialized to leader last log index + 1)
	nextIndex map[ServerId]LogIndex

	// for each server, index of highest log entry known to be replicated
	// on server
	// (initialized to 0, increases monotonically)
	matchIndex map[ServerId]LogIndex
}

// New instance set up for a fresh leader
func newLeaderVolatileState(clusterInfo *ClusterInfo, indexOfLastEntry LogIndex) *leaderVolatileState {
	lvs := &leaderVolatileState{
		make(map[ServerId]LogIndex),
		make(map[ServerId]LogIndex),
	}

	clusterInfo.ForEachPeer(
		func(peerId ServerId) {
			// #5.3-p8s4: When a leader first comes to power, it initializes
			// all nextIndex values to the index just after the last one in
			// its log (11 in Figure 7).
			lvs.nextIndex[peerId] = indexOfLastEntry + 1
			//
			lvs.matchIndex[peerId] = 0
		},
	)

	return lvs
}
