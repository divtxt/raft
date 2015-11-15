// RequestVote RPC

package raft

import (
	"fmt"
)

func (cm *passiveConsensusModule) _processRpc_RequestVote(
	fromPeer ServerId,
	rpcRequestVote *RpcRequestVote,
) bool {
	switch cm.serverState {
	case FOLLOWER:
		// Pass through to main logic below
	case CANDIDATE:
		// Pass through to main logic below
	case LEADER:
		// Pass through to main logic below
	default:
		panic(fmt.Sprintf("FATAL: unknown ServerState: %v", cm.serverState))
	}

	// 1. Reply false if term < currentTerm (#5.1)
	return false
}
