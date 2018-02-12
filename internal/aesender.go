package internal

import (
	. "github.com/divtxt/raft"
)

// IAppendEntriesSender is responsible for constructing and sending an RpcAppendEntries
// to the given peer.
//
// Concurrency: the ConsensusModule should only ever make one call at a time to this interface.
//
type IAppendEntriesSender interface {

	// SendAppendEntriesToPeerAsync constructs and sends an RpcAppendEntries for the given peer.
	//
	// The construction and sending of the RpcAppendEntries is expected to be asynchronous.
	// See RpcSendOnly.SendOnlyRpcAppendEntriesAsync.
	//
	// This method can return errors if the parameters are invalid.
	SendAppendEntriesToPeerAsync(params SendAppendEntriesParams) error
}

type SendAppendEntriesParams struct {
	PeerId        ServerId
	PeerNextIndex LogIndex
	Empty         bool
	CurrentTerm   TermNo
	CommitIndex   LogIndex
}
