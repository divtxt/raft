package raft

import (
	"fmt"
)

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

// Get nextIndex for the given peer
func (lvs *leaderVolatileState) getNextIndex(peerId ServerId) LogIndex {
	nextIndex, ok := lvs.nextIndex[peerId]
	if !ok {
		panic(fmt.Sprintf("leaderVolatileState.getNextIndex(): unknown peer: %v", peerId))
	}
	return nextIndex
}

// Decrement nextIndex for the given peer
func (lvs *leaderVolatileState) decrementNextIndex(peerId ServerId) {
	nextIndex, ok := lvs.nextIndex[peerId]
	if !ok {
		panic(fmt.Sprintf("leaderVolatileState.decrementNextIndex(): unknown peer: %v", peerId))
	}
	if nextIndex <= 1 {
		panic(fmt.Sprintf("leaderVolatileState.decrementNextIndex(): nextIndex <=1 for peer: %v", peerId))
	}
	lvs.nextIndex[peerId] = nextIndex - 1
}

// Set matchIndex for the given peer and update nextIndex to matchIndex+1
func (lvs *leaderVolatileState) setMatchIndexAndNextIndex(peerId ServerId, matchIndex LogIndex) {
	if _, ok := lvs.nextIndex[peerId]; !ok {
		panic(fmt.Sprintf("leaderVolatileState.setNextIndexAndMatchIndex(): unknown peer: %v", peerId))
	}
	lvs.nextIndex[peerId] = matchIndex + 1
	lvs.matchIndex[peerId] = matchIndex
}

// Helper method to find potential new commitIndex.
// Returns 0 if no match found.
// #RFS-L4: If there exists an N such that N > commitIndex, a majority
// of matchIndex[i] >= N, and log[N].term == currentTerm:
// set commitIndex = N (#5.3, #5.4)
func findNewerCommitIndex(
	ci *ClusterInfo,
	lvs *leaderVolatileState,
	log Log,
	currentTerm TermNo,
	commitIndex LogIndex,
) LogIndex {
	indexOfLastEntry, err := log.GetIndexOfLastEntry()
	if err != nil {
		panic(err)
	}
	requiredMatches := ci.QuorumSizeForCluster()
	// cover all N > commitIndex
	// stop when we pass the end of the log
	for N := commitIndex + 1; N <= indexOfLastEntry; N++ {
		// check log[N].term
		termAtN, err := log.GetTermAtIndex(N)
		if err != nil {
			panic(err)
		}
		if termAtN > currentTerm {
			// term has gone too high for log[N].term == currentTerm
			// no point trying further
			return 0
		}
		if termAtN < currentTerm {
			continue
		}
		// finally, check for majority of matchIndex
		var foundMatches uint = 1 // 1 because we already match!
		for _, peerMatchIndex := range lvs.matchIndex {
			if peerMatchIndex >= N {
				foundMatches++
			}
		}
		if foundMatches >= requiredMatches {
			return N
		}
	}

	return 0
}
