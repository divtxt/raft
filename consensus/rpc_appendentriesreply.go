// AppendEntriesReply RPC
// Reply to leader.

package consensus

import (
	"fmt"
	"log"

	. "github.com/divtxt/raft"
)

func (cm *PassiveConsensusModule) RpcReply_RpcAppendEntriesReply(
	from ServerId,
	appendEntries *RpcAppendEntries,
	appendEntriesReply *RpcAppendEntriesReply,
) error {
	cm.lock.Lock()
	defer cm.lock.Unlock()

	serverTerm := cm.RaftPersistentState.GetCurrentTerm()

	// Extra: ignore replies for previous term rpc
	if appendEntries.Term != serverTerm {
		return nil
	}

	// Extra: raft violation - only leader should get AppendEntriesReply
	if cm.serverState != LEADER {
		return fmt.Errorf(
			"FATAL: non-leader got AppendEntriesReply from: %v with term: %v",
			from,
			serverTerm,
		)
	}

	fm, err := cm.LeaderVolatileState.GetFollowerManager(from)
	if err != nil {
		return err
	}

	// Ignore reply for an RpcAppendEntries that does not match the current state.
	nextIndex := fm.GetNextIndex()
	expectedPrevLogIndex := nextIndex - 1
	if appendEntries.PrevLogIndex != expectedPrevLogIndex {
		log.Printf(
			"RpcReply_RpcAppendEntriesReply: Ignore due to PrevLogIndex mismatch: %d != %d",
			appendEntries.PrevLogIndex,
			expectedPrevLogIndex,
		)
		return nil
	}

	// #RFS-A2: If RPC request or response contains term T > currentTerm:
	// set currentTerm = T, convert to follower (#5.1)
	// #5.1-p3s4: ...; if one server's current term is smaller than the
	// other's, then it updates its current term to the larger value.
	// #5.1-p3s5: If a candidate or leader discovers that its term is out of
	// date, it immediately reverts to follower state.
	senderCurrentTerm := appendEntriesReply.Term
	if senderCurrentTerm > serverTerm {
		err := cm.becomeFollowerWithTerm(senderCurrentTerm)
		if err != nil {
			return err
		}
		return nil
	}

	// #RFS-L3.2: If AppendEntries fails because of log inconsistency:
	// decrement nextIndex and retry (#5.3)
	// #5.3-p8s6: After a rejection, the leader decrements nextIndex and
	// retries the AppendEntries RPC.
	if !appendEntriesReply.Success {
		err := fm.DecrementNextIndex()
		if err != nil {
			return err
		}
		err = fm.SendAppendEntriesToPeerAsync(
			false,
			serverTerm,
			cm.commitIndex.Get(),
		)
		if err != nil {
			return err
		}
		return nil
	}

	// #RFS-L3.1: If successful: update nextIndex and matchIndex for
	// follower (#5.3)
	newMatchIndex := appendEntries.PrevLogIndex + LogIndex(len(appendEntries.Entries))
	fm.SetMatchIndexAndNextIndex(newMatchIndex)

	// #RFS-L4: If there exists an N such that N > commitIndex, a majority
	// of matchIndex[i] >= N, and log[N].term == currentTerm:
	// set commitIndex = N (#5.3, #5.4)
	err = cm.advanceCommitIndexIfPossible()
	if err != nil {
		return err
	}

	return nil
}
