// RequestVote RPC

package raft

func (cm *passiveConsensusModule) _processRpc_RequestVote(
	fromPeer ServerId,
	rpcRequestVote *RpcRequestVote,
) bool {
	// 1. Reply false if term < currentTerm (#5.1)
	return false
}
