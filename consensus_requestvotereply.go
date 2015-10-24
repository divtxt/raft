// RequestVoteReply RPC
// Sent to candidates seeking election.

package raft

func (cm *passiveConsensusModule) _processRpc_RequestVoteReply(fromPeer ServerId, rpcRequestVoteReply *RpcRequestVoteReply) {

	// #5.2-p3s1: A candidate wins an election if it receives votes from a
	// majority of the servers in the full cluster for the same term.
	if rpcRequestVoteReply.VoteGranted {
		haveQuorum := cm.candidateVolatileState.addVoteFrom(fromPeer)
		if haveQuorum {
			cm.setServerState(LEADER)
			// FIXME: more leader things!
		}
	} // else FIXME

}
