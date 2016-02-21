// RequestVoteReply RPC
// Sent to candidates seeking election.

package raft

func (cm *passiveConsensusModule) rpcReply_RpcRequestVoteReply(
	fromPeer ServerId,
	rpcRequestVote *RpcRequestVote,
	rpcRequestVoteReply *RpcRequestVoteReply,
) {
	serverState := cm.getServerState()
	serverTerm := cm.persistentState.GetCurrentTerm()

	// Extra: ignore replies for previous term rpc
	if rpcRequestVote.Term != serverTerm {
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
		cm.becomeFollowerWithTerm(senderCurrentTerm)
	}

	switch serverState {
	case FOLLOWER:
		// Ignore - not a candidate
	case CANDIDATE:
		// #RFS-C2: If votes received from majority of servers: become leader
		// #5.2-p3s1: A candidate wins an election if it receives votes from a
		// majority of the servers in the full cluster for the same term.
		if rpcRequestVoteReply.VoteGranted {
			haveQuorum := cm.candidateVolatileState.addVoteFrom(fromPeer)
			if haveQuorum {
				cm.becomeLeader()
			}
		}
	case LEADER:
		// Ignore - not a candidate
	}

}
