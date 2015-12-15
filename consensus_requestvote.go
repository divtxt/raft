// RequestVote RPC

package raft

import (
	"time"
)

func (cm *passiveConsensusModule) _processRpc_RequestVote(
	serverState ServerState,
	fromPeer ServerId,
	rpcRequestVote *RpcRequestVote,
	now time.Time,
) bool {
	switch serverState {
	case FOLLOWER:
		// Pass through to main logic below
	case CANDIDATE:
		// Pass through to main logic below
	case LEADER:
		// Pass through to main logic below
	}

	serverTerm := cm.persistentState.GetCurrentTerm()
	senderCurrentTerm := rpcRequestVote.Term

	// 1. Reply false if term < currentTerm (#5.1)
	if senderCurrentTerm < serverTerm {
		return false
	}

	// #RFS-A2: If RPC request or response contains term T > currentTerm:
	// set currentTerm = T, convert to follower (#5.1)
	// #5.1-p3s4: ...; if one server's current term is smaller than the
	// other's, then it updates its current term to the larger value.
	// #5.1-p3s5: If a candidate or leader discovers that its term is out of
	// date, it immediately reverts to follower state.
	if senderCurrentTerm > serverTerm {
		cm.becomeFollowerWithTerm(senderCurrentTerm)
		serverTerm = cm.persistentState.GetCurrentTerm()
	}

	// #5.4.1-p3s1: Raft determines which of two logs is more up-to-date by
	// comparing the index and term of the last entries in the logs.
	var senderIsAtLeastAsUpToDate bool = false
	lastEntryIndex, lastEntryTerm := GetIndexAndTermOfLastEntry(cm.log)
	senderLastEntryIndex := rpcRequestVote.LastLogIndex
	senderLastEntryTerm := rpcRequestVote.LastLogTerm
	if senderLastEntryTerm != lastEntryTerm {
		// #5.4.1-p3s2: If the logs have last entries with different terms, then
		// the log with the later term is more up-to-date.
		if senderLastEntryTerm > lastEntryTerm {
			senderIsAtLeastAsUpToDate = true
		}
	} else {
		// #5.4.1-p3s3: If the logs end with the same term, then whichever log is
		// longer is more up-to-date.
		if senderLastEntryIndex >= lastEntryIndex {
			senderIsAtLeastAsUpToDate = true
		}
	}

	// 2. If votedFor is null or candidateId, and candidate's log is at least as
	// up-to-date as receiver's log, grant vote (#5.2, #5.4)
	votedFor := cm.persistentState.GetVotedFor()
	if (votedFor == "" || votedFor == fromPeer) && senderIsAtLeastAsUpToDate {
		if votedFor == "" {
			cm.persistentState.SetVotedFor(fromPeer)
		}
		// #RFS-F2: (paraphrasing) granting vote should prevent election timeout
		cm.electionTimeoutTracker.touch(now)
		return true
	}

	return false
}
