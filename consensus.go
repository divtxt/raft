package raft

type ConsensusModule struct {
	persistentState PersistentState
}

func (cm *ConsensusModule) processRpc(appendEntries AppendEntries) AppendEntriesReply {
	success := cm._processRpc_AppendEntries(appendEntries)
	return AppendEntriesReply{cm.persistentState.currentTerm, success}
}
