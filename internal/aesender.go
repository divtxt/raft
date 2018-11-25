package internal

import (
	. "github.com/divtxt/raft"
)

// IAppendEntriesSender is responsible for constructing and sending an RpcAppendEntries
// to the given peer.
//
// The implementation of this interface can optionally support sending snapshots -
// i.e. if the Log compaction prevents sending RpcAppendEntries, the implementation can
// choose to send an RpcInstallSnapshot instead.
//
// Concurrency: the ConsensusModule should only ever make one call at a time to this interface.
//
type IAppendEntriesSender interface {

	// SendAppendEntriesToPeerAsync constructs and sends an RpcAppendEntries for the given peer.
	//
	// The construction and sending of the RpcAppendEntries is expected to be asynchronous.
	// See RpcSendOnly.SendOnlyRpcAppendEntriesAsync.
	//
	// This method must return errors if the parameters are invalid, and any such error will
	// shutdown the calling ConsensusModule.
	//
	// If the Log has discarded entries at or beyond the given PeerNextIndex AND the
	// implementation does NOT support snapshots, it should return ErrIndexCompacted
	// to indicate this. Note that this will currently shutdown the ConsensusModule.
	//
	SendAppendEntriesToPeerAsync(params SendAppendEntriesParams) error
}

type SendAppendEntriesParams struct {
	PeerId        ServerId
	PeerNextIndex LogIndex
	Empty         bool
	CurrentTerm   TermNo
	CommitIndex   LogIndex
}
