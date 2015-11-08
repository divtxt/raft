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
func newCandidateVolatileState(peerServerIds []ServerId) *candidateVolatileState {
	cvs := &candidateVolatileState{}

	var clusterSize uint
	clusterSize = (uint)(len(peerServerIds) + 1)

	cvs.receivedVotes = 1 // assumes we always vote for ourself
	cvs.requiredVotes = QuorumSizeForClusterSize(clusterSize)
	cvs.votedPeers = make(map[ServerId]bool)

	for _, peerId := range peerServerIds {
		cvs.votedPeers[peerId] = false
	}

	return cvs
}

// Add a granted vote.
// Returns true if quorum has been achieved
func (cvs *candidateVolatileState) addVoteFrom(peerId ServerId) bool {
	voted, ok := cvs.votedPeers[peerId]
	if !ok {
		panic(fmt.Sprintf("candidateVolatileState.addVoteFrom(): unknown peer: %v", peerId))
	}
	if !voted {
		cvs.votedPeers[peerId] = true
		cvs.receivedVotes++
	}
	return cvs.receivedVotes >= cvs.requiredVotes
}
