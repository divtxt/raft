package raft

type ConsensusModule interface {
	// Get the current server state
	GetServerState() ServerState
}

type consensusModuleImpl struct {
	serverState     ServerState
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

func (cm *consensusModuleImpl) GetServerState() ServerState {
	return cm.serverState
}

func (cm *consensusModuleImpl) processRpc(appendEntries AppendEntries) (AppendEntriesReply, error) {
	success, err := cm._processRpc_AppendEntries(appendEntries)
	return AppendEntriesReply{cm.persistentState.GetCurrentTerm(), success}, err
}
