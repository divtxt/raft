package candidate

import (
	"fmt"

	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/config"
)

// Volatile state on candidates
type CandidateVolatileState struct {
	receivedVotes uint
	requiredVotes uint
	votedPeers    map[ServerId]bool
}

// New instance set up for a fresh election
func NewCandidateVolatileState(
	clusterInfo *config.ClusterInfo,
) *CandidateVolatileState {
	cvs := &CandidateVolatileState{
		1, // assumes we always vote for ourself
		clusterInfo.QuorumSizeForCluster(),
		make(map[ServerId]bool),
	}

	err := clusterInfo.ForEachPeer(
		func(peerId ServerId) error {
			cvs.votedPeers[peerId] = false
			return nil
		},
	)
	if err != nil {
		panic(err) // Should not happen!
	}

	return cvs
}

// Add a granted vote.
// Returns true if quorum has been achieved
func (cvs *CandidateVolatileState) AddVoteFrom(peerId ServerId) (bool, error) {
	voted, ok := cvs.votedPeers[peerId]
	if !ok {
		return false, fmt.Errorf("CandidateVolatileState.AddVoteFrom(): unknown peer: %v", peerId)
	}
	if !voted {
		cvs.votedPeers[peerId] = true
		cvs.receivedVotes++
	}
	return cvs.receivedVotes >= cvs.requiredVotes, nil
}
