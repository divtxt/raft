// Interface to this package.

package raft

type ConsensusModule interface {
	// Get the current server state
	GetServerState() ServerState

	// Process the given RPC
	ProcessRpc(AppendEntries) (AppendEntriesReply, error)

	// Election timeout occurred
	ElectionTimeout()
}

// Initialize a consensus module wrapping the given persistent state
// and log implementations
func NewConsensusModule(
	persistentState PersistentState,
	log Log,
) ConsensusModule {
	return newConsensusModuleImpl(persistentState, log, nil)
}
