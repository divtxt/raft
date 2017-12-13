package internal

import (
	. "github.com/divtxt/raft"
)

// RpcSendOnly is equivalent to RpcService without the reply part.
type RpcSendOnly interface {
	SendOnlyRpcAppendEntriesAsync(toServer ServerId, rpc *RpcAppendEntries)
	SendOnlyRpcRequestVoteAsync(toServer ServerId, rpc *RpcRequestVote)
}
