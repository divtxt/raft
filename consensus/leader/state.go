package leader

import (
	"fmt"

	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/config"
	"github.com/divtxt/raft/internal"
)

// Volatile state on leaders
// (Reinitialized after election)
type LeaderVolatileState struct {
	// for each server, index of the next log entry to send to that server
	// (initialized to leader last log index + 1)
	NextIndex map[ServerId]LogIndex

	// for each server, index of highest log entry known to be replicated
	// on server
	// (initialized to 0, increases monotonically)
	MatchIndex map[ServerId]LogIndex

	aeSender internal.IAppendEntriesSender
}

func (lvs *LeaderVolatileState) GoString() string {
	return fmt.Sprintf(
		"&LeaderVolatileState{NextIndex: %#v, MatchIndex: %#v}",
		lvs.NextIndex,
		lvs.MatchIndex,
	)
}

// New instance set up for a fresh leader
func NewLeaderVolatileState(
	clusterInfo *config.ClusterInfo,
	indexOfLastEntry LogIndex,
	aeSender internal.IAppendEntriesSender,
) (*LeaderVolatileState, error) {
	lvs := &LeaderVolatileState{
		make(map[ServerId]LogIndex),
		make(map[ServerId]LogIndex),
		aeSender,
	}

	err := clusterInfo.ForEachPeer(
		func(peerId ServerId) error {
			// #5.3-p8s4: When a leader first comes to power, it initializes
			// all nextIndex values to the index just after the last one in
			// its log (11 in Figure 7).
			lvs.NextIndex[peerId] = indexOfLastEntry + 1
			//
			lvs.MatchIndex[peerId] = 0

			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	return lvs, nil
}

// Get nextIndex for the given peer
func (lvs *LeaderVolatileState) GetNextIndex(peerId ServerId) (LogIndex, error) {
	nextIndex, ok := lvs.NextIndex[peerId]
	if !ok {
		return 0, fmt.Errorf("LeaderVolatileState.GetNextIndex(): unknown peer: %v", peerId)
	}
	return nextIndex, nil
}

// Decrement nextIndex for the given peer
func (lvs *LeaderVolatileState) DecrementNextIndex(peerId ServerId) error {
	nextIndex, ok := lvs.NextIndex[peerId]
	if !ok {
		return fmt.Errorf("LeaderVolatileState.DecrementNextIndex(): unknown peer: %v", peerId)
	}
	if nextIndex <= 1 {
		return fmt.Errorf("LeaderVolatileState.DecrementNextIndex(): nextIndex <=1 for peer: %v", peerId)
	}
	lvs.NextIndex[peerId] = nextIndex - 1
	return nil
}

// Set matchIndex for the given peer and update nextIndex to matchIndex+1
func (lvs *LeaderVolatileState) SetMatchIndexAndNextIndex(peerId ServerId, matchIndex LogIndex) error {
	if _, ok := lvs.NextIndex[peerId]; !ok {
		return fmt.Errorf("LeaderVolatileState.SetMatchIndexAndNextIndex(): unknown peer: %v", peerId)
	}
	lvs.NextIndex[peerId] = matchIndex + 1
	lvs.MatchIndex[peerId] = matchIndex
	return nil
}

// Construct and send RpcAppendEntries to the given peer.
func (lvs *LeaderVolatileState) SendAppendEntriesToPeerAsync(
	peerId ServerId,
	empty bool,
	currentTerm TermNo,
	commitIndex LogIndex,
) error {
	//
	peerNextIndex, err := lvs.GetNextIndex(peerId)
	if err != nil {
		return err
	}
	//
	return lvs.aeSender.SendAppendEntriesToPeerAsync(
		internal.SendAppendEntriesParams{
			peerId,
			peerNextIndex,
			empty,
			currentTerm,
			commitIndex,
		},
	)
}

// Helper method to find potential new commitIndex.
// Returns the highest N possible that is higher than currentCommitIndex.
// Returns 0 if no match found.
// #RFS-L4: If there exists an N such that N > commitIndex, a majority
// of matchIndex[i] >= N, and log[N].term == currentTerm:
// set commitIndex = N (#5.3, #5.4)
func FindNewerCommitIndex(
	ci *config.ClusterInfo,
	lvs *LeaderVolatileState,
	log LogReadOnly,
	currentTerm TermNo,
	currentCommitIndex LogIndex,
) (LogIndex, error) {
	indexOfLastEntry, err := log.GetIndexOfLastEntry()
	if err != nil {
		return 0, err
	}
	requiredMatches := ci.QuorumSizeForCluster()
	var matchingN LogIndex = 0
	// cover all N > currentCommitIndex
	// stop when we pass the end of the log
	for N := currentCommitIndex + 1; N <= indexOfLastEntry; N++ {
		// check log[N].term
		termAtN, err := log.GetTermAtIndex(N)
		if err != nil {
			return 0, err
		}
		if termAtN > currentTerm {
			// term has gone too high for log[N].term == currentTerm
			// no point trying further
			break
		}
		if termAtN < currentTerm {
			continue
		}
		// finally, check for majority of matchIndex
		var foundMatches uint = 1 // 1 because we already match!
		for _, peerMatchIndex := range lvs.MatchIndex {
			if peerMatchIndex >= N {
				foundMatches++
			}
		}
		if foundMatches >= requiredMatches {
			matchingN = N
		}
	}

	return matchingN, nil
}
