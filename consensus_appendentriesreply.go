// AppendEntriesReply RPC
// Reply to leader.

package raft

func (cm *passiveConsensusModule) _processRpc_AppendEntriesReply(
	serverState ServerState,
	from ServerId,
	appendEntries *RpcAppendEntries,
	appendEntriesReply *RpcAppendEntriesReply,
) {
	serverTerm := cm.persistentState.GetCurrentTerm()

	// Ignore reply - rpc was for a previous term
	if appendEntries.Term != serverTerm {
		return
	}

	switch serverState {
	case FOLLOWER:
		// Do nothing since this server is already a follower.
		return
	case CANDIDATE:
		// Do nothing since this server is already a follower.
		return
	case LEADER:
		// Pass through to main logic below
		panic("TODO: _processRpc_AppendEntriesReply / LEADER")
	}

}
