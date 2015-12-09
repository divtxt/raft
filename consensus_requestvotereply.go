// RequestVoteReply RPC
// Sent to candidates seeking election.

package raft

func (cm *passiveConsensusModule) _processRpc_RequestVoteReply(
	serverState ServerState,
	fromPeer ServerId,
	rpcRequestVote *RpcRequestVote,
	rpcRequestVoteReply *RpcRequestVoteReply,
) {
	serverTerm := cm.persistentState.GetCurrentTerm()

	// Ignore reply - rpc was for a previous term
	if rpcRequestVote.Term != serverTerm {
		return
	}

	switch serverState {
	case FOLLOWER:
		// Do nothing since this server is already a follower.
		// FIXME: is this correct?
		return
	case CANDIDATE:
		// Pass through to main logic below
	case LEADER:
		// Do nothing since this candidate has already won the election.
		return
	}

	// #RFS-A2: If RPC request or response contains term T > currentTerm:
	// set currentTerm = T, convert to follower (#5.1)
	// #5.1-p3s4: ...; if one server's current term is smaller than the
	// other's, then it updates its current term to the larger value.
	// #5.1-p3s5: If a candidate or leader discovers that its term is out of
	// date, it immediately reverts to follower state.
	senderCurrentTerm := rpcRequestVoteReply.Term
	if senderCurrentTerm > serverTerm {
		cm.becomeFollower(senderCurrentTerm)
	}

	// #5.2-p3s1: A candidate wins an election if it receives votes from a
	// majority of the servers in the full cluster for the same term.
	if rpcRequestVoteReply.VoteGranted {
		haveQuorum := cm.candidateVolatileState.addVoteFrom(fromPeer)
		if haveQuorum {
			cm.becomeLeader()
		}
	} // else TODO
}
