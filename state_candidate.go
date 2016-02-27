package raft

import (
	"fmt"
)

// Volatile state on candidates
type candidateVolatileState struct {
	receivedVotes uint
	requiredVotes uint
	votedPeers    map[ServerId]bool
}

// New instance set up for a fresh election
func newCandidateVolatileState(clusterInfo *ClusterInfo) *candidateVolatileState {
	cvs := &candidateVolatileState{}

	cvs.receivedVotes = 1 // assumes we always vote for ourself
	cvs.requiredVotes = clusterInfo.QuorumSizeForCluster()
	cvs.votedPeers = make(map[ServerId]bool)

	clusterInfo.ForEachPeer(
		func(peerId ServerId) {
			cvs.votedPeers[peerId] = false
		},
	)

	return cvs
}

// Add a granted vote.
// Returns true if quorum has been achieved
func (cvs *candidateVolatileState) addVoteFrom(peerId ServerId) (bool, error) {
	voted, ok := cvs.votedPeers[peerId]
	if !ok {
		return false, fmt.Errorf("candidateVolatileState.addVoteFrom(): unknown peer: %v", peerId)
	}
	if !voted {
		cvs.votedPeers[peerId] = true
		cvs.receivedVotes++
	}
	return cvs.receivedVotes >= cvs.requiredVotes, nil
}
