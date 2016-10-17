// Interface to the Raft Consensus Module.

package raft

// The Raft ConsensusModule.
type IConsensusModule interface {
	// Start the ConsensusModule running with the given ChangeListener.
	//
	// This starts a goroutine that drives ticks.
	//
	// Should only be called once.
	Start(changeListener ChangeListener) error

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

	// Append the given command as an entry in the log.
	//
	// This can only be done if the ConsensusModule is in LEADER state.
	//
	// The command will be sent to Log.AppendEntry().
	//
	// The command must already have been checked to ensure that it will successfully apply to the
	// state machine in it's position in the Log.
	//
	// Returns the index of the new entry.
	//
	// Returns ErrStopped if ConsensusModule is stopped.
	// Returns ErrNotLeader if not currently the leader.
	//
	// Any errors from Log.AppendCommand() call will stop the ConsensusModule.
	//
	// Here, we intentionally punt on some of the leader details, specifically
	// most of:
	//
	// #RFS-L2: If command received from client: append entry to local log,
	// respond after entry applied to state machine (#5.3)
	//
	// We choose not to deal with the client directly. You must implement the interaction with
	// clients and, if required, with waiting for the entry to be applied to the state machine.
	// (see delegation of lastApplied to the state machine via the ChangeListener interface)
	//
	// See the notes on NewConsensusModule() for more details about this method's behavior.
	AppendCommand(command Command) (LogIndex, error)
}
