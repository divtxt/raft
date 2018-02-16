package leader

import (
	"fmt"

	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/internal"
)

type FollowerManager struct {
	peerId ServerId

	// for each server, index of the next log entry to send to that server
	// (initialized to leader last log index + 1)
	nextIndex LogIndex

	// for each server, index of highest log entry known to be replicated
	// on server
	// (initialized to 0, increases monotonically)
	matchIndex LogIndex

	aeSender internal.IAppendEntriesSender
}

func (fm *FollowerManager) GoString() string {
	return fmt.Sprintf(
		"&FollowerManager{peerId: %d, nextIndex: %d, matchIndex: %d}",
		fm.peerId,
		fm.nextIndex,
		fm.matchIndex,
	)
}

func NewFollowerManager(
	peerId ServerId,
	nextIndex LogIndex,
	matchIndex LogIndex,
	aeSender internal.IAppendEntriesSender,
) *FollowerManager {
	return &FollowerManager{
		peerId,
		nextIndex,
		matchIndex,
		aeSender,
	}
}

func (fm *FollowerManager) getNextIndex() LogIndex {
	return fm.nextIndex
}

func (fm *FollowerManager) getMatchIndex() LogIndex {
	return fm.matchIndex
}

// Decrement nextIndex for the given peer
func (fm *FollowerManager) decrementNextIndex() error {
	if fm.nextIndex <= 1 {
		return fmt.Errorf(
			"FollowerManager.DecrementNextIndex(): nextIndex already <=1 for peer: %v",
			fm.peerId,
		)
	}
	fm.nextIndex = fm.nextIndex - 1
	return nil
}

// Set matchIndex for the given peer and update nextIndex to matchIndex+1
func (fm *FollowerManager) SetMatchIndexAndNextIndex(matchIndex LogIndex) {
	fm.nextIndex = matchIndex + 1
	fm.matchIndex = matchIndex
}

// Construct and send RpcAppendEntries to the given peer.
func (fm *FollowerManager) SendAppendEntriesToPeerAsync(
	empty bool,
	currentTerm TermNo,
	commitIndex LogIndex,
) error {
	return fm.aeSender.SendAppendEntriesToPeerAsync(
		internal.SendAppendEntriesParams{
			fm.peerId,
			fm.nextIndex,
			empty,
			currentTerm,
			commitIndex,
		},
	)
}
