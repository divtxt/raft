package aesender

import (
	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/internal"
)

// LogOnlyAESender is an implementation of AppendEntriesSender that can
// only construct RpcAppendEntries from the raft log.
// It is unable to handle raft snapshots.
type LogOnlyAESender struct {
	logRO                         internal.LogTailOnlyRO
	sendOnlyRpcAppendEntriesAsync internal.SendOnlyRpcAppendEntriesAsync
}

func NewLogOnlyAESender(
	logRO internal.LogTailOnlyRO,
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
		iole, err := s.logRO.GetIndexOfLastEntry()
		if err != nil {
			return err
		}
		indexToSend := peerLastLogIndex + 1
		if indexToSend > iole {
			entriesToSend = []LogEntry{}
		} else {
			entry, err := s.logRO.GetEntryAtIndex(indexToSend)
			if err != nil {
				return err
			}
			entriesToSend = []LogEntry{entry}
		}
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
