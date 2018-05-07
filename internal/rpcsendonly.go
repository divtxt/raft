package internal

import (
	. "github.com/divtxt/raft"
)

// SendOnlyRpcAppendEntriesAsync is equivalent to an async RpcService.RpcAppendEntries().
type SendOnlyRpcAppendEntriesAsync func(toServer ServerId, rpc *RpcAppendEntries)

// SendOnlyRpcRequestVoteAsync is equivalent to an async RpcService.RpcRequestVote().
type SendOnlyRpcRequestVoteAsync func(toServer ServerId, rpc *RpcRequestVote)
