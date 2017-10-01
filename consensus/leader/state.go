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
	followerManagers map[ServerId]*FollowerManager

	aeSender internal.IAppendEntriesSender
}

func (lvs *LeaderVolatileState) GoString() string {
	return fmt.Sprintf(
		"&LeaderVolatileState{followerManagers: %#v}",
		lvs.followerManagers,
	)
}

// New instance set up for a fresh leader
func NewLeaderVolatileState(
	clusterInfo *config.ClusterInfo,
	indexOfLastEntry LogIndex,
	aeSender internal.IAppendEntriesSender,
) (*LeaderVolatileState, error) {
	lvs := &LeaderVolatileState{
		make(map[ServerId]*FollowerManager),
		aeSender,
	}

	err := clusterInfo.ForEachPeer(
		func(peerId ServerId) error {
			// #5.3-p8s4: When a leader first comes to power, it initializes
			// all nextIndex values to the index just after the last one in
			// its log (11 in Figure 7).
			fm := NewFollowerManager(
				peerId,
				indexOfLastEntry+1,
				0,
				aeSender,
			)
			lvs.followerManagers[peerId] = fm

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
	fm, ok := lvs.followerManagers[peerId]
	if !ok {
		return 0, fmt.Errorf("LeaderVolatileState.GetNextIndex(): unknown peer: %v", peerId)
	}
	return fm.getNextIndex(), nil
}

// Decrement nextIndex for the given peer
func (lvs *LeaderVolatileState) DecrementNextIndex(peerId ServerId) error {
	fm, ok := lvs.followerManagers[peerId]
	if !ok {
		return fmt.Errorf("LeaderVolatileState.DecrementNextIndex(): unknown peer: %v", peerId)
	}
	return fm.decrementNextIndex()
}

// Set matchIndex for the given peer and update nextIndex to matchIndex+1
func (lvs *LeaderVolatileState) SetMatchIndexAndNextIndex(peerId ServerId, matchIndex LogIndex) error {
	fm, ok := lvs.followerManagers[peerId]
	if !ok {
		return fmt.Errorf("LeaderVolatileState.SetMatchIndexAndNextIndex(): unknown peer: %v", peerId)
	}
	fm.SetMatchIndexAndNextIndex(matchIndex)
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

// Helper for tests
func (lvs *LeaderVolatileState) NextIndexes() map[ServerId]LogIndex {
	m := make(map[ServerId]LogIndex)
	for peerId, fm := range lvs.followerManagers {
		m[peerId] = fm.getNextIndex()
	}
	return m
}

// Helper for tests
func (lvs *LeaderVolatileState) MatchIndexes() map[ServerId]LogIndex {
	m := make(map[ServerId]LogIndex)
	for peerId, fm := range lvs.followerManagers {
		m[peerId] = fm.getMatchIndex()
	}
	return m
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
		for _, fm := range lvs.followerManagers {
			if fm.getMatchIndex() >= N {
				foundMatches++
			}
		}
		if foundMatches >= requiredMatches {
			matchingN = N
		}
	}

	return matchingN, nil
}
