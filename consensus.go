package raft

type ConsensusModule struct {
	persistentState PersistentState
}

func (cm *ConsensusModule) processRpc(appendEntries AppendEntries) (AppendEntriesReply, error) {
	success, err := cm._processRpc_AppendEntries(appendEntries)
	return AppendEntriesReply{cm.persistentState.currentTerm, success}, err
}
