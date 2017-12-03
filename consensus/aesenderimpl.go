package consensus

import (
	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/internal"
)

type appendEntriesSender struct {
	logRO       LogReadOnly
	rpcSendOnly internal.RpcSendOnly
}

func (aes *appendEntriesSender) SendAppendEntriesToPeerAsync(
	params internal.SendAppendEntriesParams,
) {
	peerLastLogIndex := params.PeerNextIndex - 1
	//
	var peerLastLogTerm TermNo
	if peerLastLogIndex == 0 {
		peerLastLogTerm = 0
	} else {
		var err error
		peerLastLogTerm, err = aes.logRO.GetTermAtIndex(peerLastLogIndex)
		if err != nil {
			return
		}
	}
	//
	var entriesToSend []LogEntry
	if params.Empty {
		entriesToSend = []LogEntry{}
	} else {
		var err error
		entriesToSend, err = aes.logRO.GetEntriesAfterIndex(peerLastLogIndex)
		if err != nil {
			return
		}
	}
	//
	rpcAppendEntries := &RpcAppendEntries{
		params.CurrentTerm,
		peerLastLogIndex,
		peerLastLogTerm,
		entriesToSend,
		params.CommitIndex,
	}
	aes.rpcSendOnly.SendOnlyRpcAppendEntriesAsync(params.PeerId, rpcAppendEntries)
}
