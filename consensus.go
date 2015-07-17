package raft

type ConsensusModule struct {
	persistentState PersistentState
}

func (cm ConsensusModule) processRpc(appendEntries AppendEntries) AppendEntriesReply {
	success := _processRpc_AppendEntries(appendEntries)
	return AppendEntriesReply{cm.persistentState.currentTerm, success}
}

func _processRpc_AppendEntries(appendEntries AppendEntries) bool {
	return false
}
