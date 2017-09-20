package candidate

import (
	"fmt"

	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/config"
)

// Volatile state on candidates
type CandidateVolatileState struct {
	ReceivedReplies uint
	ReceivedVotes   uint
	QuorumSize      uint
	VotedPeers      map[ServerId]bool
}

// New instance set up for a fresh election
func NewCandidateVolatileState(
	clusterInfo *config.ClusterInfo,
) (*CandidateVolatileState, error) {
	cvs := &CandidateVolatileState{
		1, // assumes we always vote for ourself
		1, // assumes we always vote for ourself
		clusterInfo.QuorumSizeForCluster(),
		make(map[ServerId]bool),
	}

	err := clusterInfo.ForEachPeer(
		func(peerId ServerId) error {
			cvs.VotedPeers[peerId] = false
			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	return cvs, nil
}

// Add a vote reply.
// Returns true if quorum has been achieved
func (cvs *CandidateVolatileState) AddVoteFrom(
	peerId ServerId, voteGranted bool,
) (bool, error) {
	voted, ok := cvs.VotedPeers[peerId]
	if !ok {
		return false, fmt.Errorf("CandidateVolatileState.AddVoteFrom(): unknown peer: %v", peerId)
	}
	if !voted {
		cvs.VotedPeers[peerId] = true
		cvs.ReceivedReplies++
		if voteGranted {
			cvs.ReceivedVotes++
		}
	}
	return cvs.ReceivedVotes >= cvs.QuorumSize, nil
}

// Check if we got replies from a quorum of the cluster.
func (cvs *CandidateVolatileState) GotQuorumReplies() bool {
	return cvs.ReceivedReplies >= cvs.QuorumSize
}
