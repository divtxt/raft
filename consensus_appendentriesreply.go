// AppendEntriesReply RPC
// Reply to leader.

package raft

import (
	"fmt"
)

func (cm *passiveConsensusModule) _processRpc_AppendEntriesReply(
	serverState ServerState,
	from ServerId,
	appendEntries *RpcAppendEntries,
	appendEntriesReply *RpcAppendEntriesReply,
) {
	serverTerm := cm.persistentState.GetCurrentTerm()

	// Extra: ignore replies for previous term rpc
	if appendEntries.Term != serverTerm {
		return
	}

	switch serverState {
	case FOLLOWER:
		// Extra: raft violation - only leader should get AppendEntriesReply
		fallthrough
	case CANDIDATE:
		// Extra: raft violation - only leader should get AppendEntriesReply
		panic(fmt.Sprintf(
			"FATAL: non-leader got AppendEntriesReply from: %v with term: %v",
			from,
			serverTerm,
		))
	case LEADER:
		// Pass through to main logic below
	}

	// #RFS-A2: If RPC request or response contains term T > currentTerm:
	// set currentTerm = T, convert to follower (#5.1)
	// #5.1-p3s4: ...; if one server's current term is smaller than the
	// other's, then it updates its current term to the larger value.
	// #5.1-p3s5: If a candidate or leader discovers that its term is out of
	// date, it immediately reverts to follower state.
	senderCurrentTerm := appendEntriesReply.Term
	if senderCurrentTerm > serverTerm {
		cm.becomeFollowerWithTerm(senderCurrentTerm)
		// TODO: test for this
		return
	}

	// #RFS-L3.2: If AppendEntries fails because of log inconsistency:
	// decrement nextIndex and retry (#5.3)
	// #5.3-p8s6: After a rejection, the leader decrements nextIndex and
	// retries the AppendEntries RPC.
	if !appendEntriesReply.Success {
		cm.leaderVolatileState.decrementNextIndex(from)
		cm.sendAppendEntriesToPeer(from, false)
		// TODO: test for this
		return
	}

	// #RFS-L3.1: If successful: update nextIndex and matchIndex for
	// follower (#5.3)
	newMatchIndex := appendEntries.PrevLogIndex + LogIndex(len(appendEntries.Entries))
	cm.leaderVolatileState.setMatchIndexAndNextIndex(from, newMatchIndex)
}
