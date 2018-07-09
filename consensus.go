// Interface to the Raft Consensus Module.

package raft

// The Raft ConsensusModule.
type IConsensusModule interface {

	// Check if the ConsensusModule is stopped.
	IsStopped() bool

	// Stop the ConsensusModule.
	//
	// This is safe to call multiple times, even if the ConsensusModule has already stopped.
	Stop()

	// Get the current server state.
	//
	// If the ConsensusModule is stopped this is the state when it stopped.
	GetServerState() ServerState

	// Process the given RpcAppendEntries message from the given peer.
	//
	// Returns nil if there was an error or if the ConsensusModule is shutdown.
	//
	// Note that an error would have shutdown the ConsensusModule.
	ProcessRpcAppendEntries(from ServerId, rpc *RpcAppendEntries) *RpcAppendEntriesReply

	// Process the given RpcRequestVote message from the given peer.
	//
	// Returns nil if there was an error or if the ConsensusModule is shutdown.
	//
	// Note that an error would have shutdown the ConsensusModule.
	ProcessRpcRequestVote(from ServerId, rpc *RpcRequestVote) *RpcRequestVoteReply

	// AppendCommand appends the given serialized command to the Raft log and applies it
	// to the state machine once it is considered committed by the ConsensusModule.
	//
	// This can only be done if the ConsensusModule is in LEADER state.
	//
	// When the command has been replicated to enough followers and is considered committed it is
	// applied to the state machine. The value returned by the state machine is then sent on the
	// channel returned by this method.
	//
	// If the ConsensusModule loses leader status before this entry commits, and the new leader
	// overwrites the given command in the log, the channel will be closed without a value
	// being sent.
	//
	// Returns ErrStopped if ConsensusModule is stopped.
	// Returns ErrNotLeader if not currently the leader.
	//
	// #RFS-L2: If command received from client: append entry to local log,
	// respond after entry applied to state machine (#5.3)
	//
	// See the notes on NewConsensusModule() for more details about this method's behavior.
	AppendCommand(command Command) (<-chan CommandResult, error)
}

// A subset of the IConsensusModule interface with just the AppendCommand method.
type IConsensusModule_AppendCommandOnly interface {
	AppendCommand(command Command) (<-chan CommandResult, error)
}
