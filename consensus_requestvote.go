// RequestVote RPC

package raft

func (cm *passiveConsensusModule) _processRpc_RequestVote(
	serverState ServerState,
	fromPeer ServerId,
	rpcRequestVote *RpcRequestVote,
) bool {
	switch serverState {
	case FOLLOWER:
		// Pass through to main logic below
	case CANDIDATE:
		// Pass through to main logic below
	case LEADER:
		// Pass through to main logic below
	}

	// 1. Reply false if term < currentTerm (#5.1)
	return false
}
