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
		panic("TODO: _processRpc_AppendEntriesReply / LEADER")
	}
}
