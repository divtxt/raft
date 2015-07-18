// AppendEntries RPC (Receiver Implementation)
// Invoked by leader to replicate log entries (#5.3); also used as heartbeat
// (#5.2).

package raft

func _processRpc_AppendEntries(appendEntries AppendEntries) bool {
	// 1. Reply false if term < currentTerm (#5.1)
	return false
}
