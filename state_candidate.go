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
func newCandidateVolatileState(
	clusterInfo *ClusterInfo,
) (*candidateVolatileState, error) {
	cvs := &candidateVolatileState{}

	cvs.receivedVotes = 1 // assumes we always vote for ourself
	cvs.requiredVotes = clusterInfo.QuorumSizeForCluster()
	cvs.votedPeers = make(map[ServerId]bool)

	err := clusterInfo.ForEachPeer(
		func(peerId ServerId) error {
			cvs.votedPeers[peerId] = false
			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	return cvs, nil
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
