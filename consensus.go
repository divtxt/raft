package raft

type ConsensusModule interface {
	// Get the current server state
	GetServerState() ServerState

	// Process the given RPC
	ProcessRpc(AppendEntries) (AppendEntriesReply, error)
}

// Initialize a consensus module with the given persistent state
func NewConsensusModule(persistentState PersistentState, log Log) ConsensusModule {
	return newConsensusModuleImpl(persistentState, log)
}

// Internal implementation structure
type consensusModuleImpl struct {
	serverState     ServerState
	persistentState PersistentState
	log             Log
	volatileState   VolatileState
}

// Expose actual type for tests to use
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

func (cm *consensusModuleImpl) ProcessRpc(appendEntries AppendEntries) (AppendEntriesReply, error) {
	success, err := cm._processRpc_AppendEntries(appendEntries)
	return AppendEntriesReply{cm.persistentState.GetCurrentTerm(), success}, err
}
