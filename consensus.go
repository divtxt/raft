package raft

// Internal implementation structure
type consensusModuleImpl struct {
	serverState     ServerState
	persistentState PersistentState
	log             Log
	volatileState   VolatileState
}

// Internal interface for timeout callbacks
type timeoutHelper interface {
	resetElectionTimeout()
}

// Expose actual type for tests to use
func newConsensusModuleImpl(
	persistentState PersistentState,
	log Log,
	timeoutHelper timeoutHelper,
) *consensusModuleImpl {
	timeoutHelper.resetElectionTimeout()
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

// #5.2-p1s5: If a follower receives no communication over a period of time
// called the election timeout, then it assumes there is no viable leader
// and begins an election to choose a new leader.
// #5.2-p2s1: To begin an election, a follower increments its current term
// and transitions to candidate state.
func (cm *consensusModuleImpl) ElectionTimeout() {
	cm.persistentState.SetCurrentTerm(cm.persistentState.GetCurrentTerm() + 1)
	cm.serverState = CANDIDATE
}
