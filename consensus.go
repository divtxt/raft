package raft

type ConsensusModule interface {
	GetServerMode() ServerMode
}

type consensusModuleImpl struct {
	serverMode      ServerMode
	persistentState PersistentState
	log             Log
	volatileState   VolatileState
}

// Initialize a consensus module with the given persistent state
func NewConsensusModule(persistentState PersistentState, log Log) ConsensusModule {
	return newConsensusModuleImpl(persistentState, log)
}

func newConsensusModuleImpl(persistentState PersistentState, log Log) *consensusModuleImpl {
	return &consensusModuleImpl{
		// #5.2-p1s2: When servers start up, they begin as followers
		FOLLOWER,
		persistentState,
		log,
		VolatileState{},
	}
}

func (cm *consensusModuleImpl) GetServerMode() ServerMode {
	return cm.serverMode
}

func (cm *consensusModuleImpl) processRpc(appendEntries AppendEntries) (AppendEntriesReply, error) {
	success, err := cm._processRpc_AppendEntries(appendEntries)
	return AppendEntriesReply{cm.persistentState.GetCurrentTerm(), success}, err
}
