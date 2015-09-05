package raft

// Volatile state on candidates
type CandidateVolatileState struct {
	receivedVotes uint
	requiredVotes uint
	votedPeers    map[ServerId]bool
}

// New instance set up for a fresh election
func newCandidateVolatileState(peerServerIds []ServerId) *CandidateVolatileState {
	cvs := &CandidateVolatileState{}

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
func (cvs *CandidateVolatileState) addVoteFrom(peerId ServerId) bool {

	if cvs.votedPeers[peerId] {
		// FIXME
		panic("this peer has already voted!")
	} else {
		cvs.votedPeers[peerId] = true
		cvs.receivedVotes++
		return cvs.receivedVotes >= cvs.requiredVotes
	}
}
