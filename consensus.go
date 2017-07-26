// Interface to the Raft Consensus Module.

package raft

// The Raft ConsensusModule.
type IConsensusModule interface {

	// Check if the ConsensusModule is stopped.
	IsStopped() bool

	// Stop the ConsensusModule.
	//
	// This will effectively stop the goroutine that does the processing.
	// This is safe to call multiple times, even if the ConsensusModule has already stopped.
	Stop()

	// Get the current server state.
	GetServerState() ServerState

	// Process the given RpcAppendEntries message from the given peer.
	//
	// Returns nil if there was an error or if the ConsensusModule is shutdown.
	//
	// Note that an error would have shutdown the ConsensusModule.
	ProcessRpcAppendEntries(from ServerId, rpc *RpcAppendEntries) *RpcAppendEntriesReply

	// Process the given RpcRequestVote message from the given peer
	// asynchronously.
	//
	// This method sends the RPC message to the ConsensusModule's goroutine.
	// The RPC reply will be sent later on the returned channel.
	//
	// See the RpcService interface for outgoing RPC.
	//
	// See the notes on NewConsensusModule() for more details about this method's behavior.
	ProcessRpcRequestVote(from ServerId, rpc *RpcRequestVote) *RpcRequestVoteReply

	// AppendCommand appends the given serialized command to the log.
	//
	// This can only be done if the ConsensusModule is in LEADER state.
	//
	// The command will be sent to Log.AppendEntry() and will wait for it to finish.
	// Any errors from Log.AppendEntry() call will stop the ConsensusModule.
	//
	// It will return a channel on which the result will later be sent.
	//
	// This method does NOT wait for the log entry to be committed by raft.
	//
	// When the command is eventually committed to the raft log, it is then applied to the state
	// machine and the value returned by the state machine will be sent on the channel.
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
	AppendCommand(command Command) (LogIndex, error)
}
