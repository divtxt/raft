// RequestVote RPC

package consensus

import (
	"fmt"
	"time"

	. "github.com/divtxt/raft"
)

// Process the given RpcRequestVote message
// #RFS-F1: Respond to RPCs from candidates and leaders
func (cm *PassiveConsensusModule) Rpc_RpcRequestVote(
	from ServerId,
	rpcRequestVote *RpcRequestVote,
	now time.Time,
) (*RpcRequestVoteReply, error) {
	if from == cm.ClusterInfo.GetThisServerId() {
		return nil, fmt.Errorf("FATAL: from server has same serverId: %v", cm.ClusterInfo.GetThisServerId())
	}

	makeReply := func(voteGranted bool) *RpcRequestVoteReply {
		return &RpcRequestVoteReply{
			cm.RaftPersistentState.GetCurrentTerm(), // refetch in case it has changed!
			voteGranted,
		}
	}

	serverTerm := cm.RaftPersistentState.GetCurrentTerm()
	senderCurrentTerm := rpcRequestVote.Term

	// 1. Reply false if term < currentTerm (#5.1)
	if senderCurrentTerm < serverTerm {
		return makeReply(false), nil
	}

	// #RFS-A2: If RPC request or response contains term T > currentTerm:
	// set currentTerm = T, convert to follower (#5.1)
	// #5.1-p3s4: ...; if one server's current term is smaller than the
	// other's, then it updates its current term to the larger value.
	// #5.1-p3s5: If a candidate or leader discovers that its term is out of
	// date, it immediately reverts to follower state.
	if senderCurrentTerm > serverTerm {
		err := cm.becomeFollowerWithTerm(senderCurrentTerm)
		if err != nil {
			return nil, err
		}
		serverTerm = cm.RaftPersistentState.GetCurrentTerm()
	}

	// #5.4.1-p3s1: Raft determines which of two logs is more up-to-date by
	// comparing the index and term of the last entries in the logs.
	var senderIsAtLeastAsUpToDate bool = false
	lastEntryIndex, lastEntryTerm, err := GetIndexAndTermOfLastEntry(cm.LogRO)
	if err != nil {
		return nil, err
	}
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
	votedFor := cm.RaftPersistentState.GetVotedFor()
	if (votedFor == 0 || votedFor == from) && senderIsAtLeastAsUpToDate {
		if votedFor == 0 {
			err = cm.RaftPersistentState.SetVotedFor(from)
			if err != nil {
				return nil, err
			}
		}
		// #RFS-F2: (paraphrasing) granting vote should prevent election timeout
		cm.ElectionTimeoutTracker.Touch(now)
		return makeReply(true), nil
	}

	return makeReply(false), nil
}
