// RequestVote RPC

package raft

func (cm *passiveConsensusModule) _processRpc_RequestVote(
	fromPeer ServerId,
	rpcRequestVote *RpcRequestVote,
) bool {
	switch cm.getServerState() {
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
