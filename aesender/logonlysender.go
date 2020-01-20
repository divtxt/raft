package aesender

import (
	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/internal"
)

// LogOnlyAESender is an implementation of AppendEntriesSender that can
// only construct RpcAppendEntries from the raft log.
// It is unable to handle raft snapshots.
type LogOnlyAESender struct {
	logRO                         internal.LogTailRO
	sendOnlyRpcAppendEntriesAsync internal.SendOnlyRpcAppendEntriesAsync
}

func NewLogOnlyAESender(
	logRO internal.LogTailRO,
	sendOnlyRpcAppendEntriesAsync internal.SendOnlyRpcAppendEntriesAsync,
) internal.IAppendEntriesSender {
	return &LogOnlyAESender{logRO, sendOnlyRpcAppendEntriesAsync}
}

func (s *LogOnlyAESender) SendAppendEntriesToPeerAsync(
	params internal.SendAppendEntriesParams,
) error {
	peerLastLogIndex := params.PeerNextIndex - 1
	//
	var peerLastLogTerm TermNo
	if peerLastLogIndex == 0 {
		peerLastLogTerm = 0
	} else {
		var err error
		peerLastLogTerm, err = s.logRO.GetTermAtIndex(peerLastLogIndex)
		if err != nil {
			return err
		}
	}
	//
	var entriesToSend []LogEntry
	if params.Empty {
		entriesToSend = []LogEntry{}
	} else {
		var err error
		entriesToSend, err = s.logRO.GetEntriesAfterIndex(peerLastLogIndex)
		if err != nil {
			return err
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
	s.sendOnlyRpcAppendEntriesAsync(params.PeerId, rpcAppendEntries)
	return nil
}
