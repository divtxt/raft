// RequestVoteReply RPC
// Sent to candidates seeking election.

package consensus

import (
	. "github.com/divtxt/raft"
)

func (cm *PassiveConsensusModule) RpcReply_RpcRequestVoteReply(
	fromPeer ServerId,
	rpcRequestVote *RpcRequestVote,
	rpcRequestVoteReply *RpcRequestVoteReply,
) error {
	serverState := cm.GetServerState()
	serverTerm := cm.RaftPersistentState.GetCurrentTerm()

	// Extra: ignore replies for previous term rpc
	if rpcRequestVote.Term != serverTerm {
		return nil
	}

	// #RFS-A2: If RPC request or response contains term T > currentTerm:
	// set currentTerm = T, convert to follower (#5.1)
	// #5.1-p3s4: ...; if one server's current term is smaller than the
	// other's, then it updates its current term to the larger value.
	// #5.1-p3s5: If a candidate or leader discovers that its term is out of
	// date, it immediately reverts to follower state.
	senderCurrentTerm := rpcRequestVoteReply.Term
	if senderCurrentTerm > serverTerm {
		err := cm.becomeFollowerWithTerm(senderCurrentTerm)
		if err != nil {
			return err
		}
	}

	if serverState == CANDIDATE {
		// #RFS-C2: If votes received from majority of servers: become leader
		// #5.2-p3s1: A candidate wins an election if it receives votes from a
		// majority of the servers in the full cluster for the same term.
		haveQuorum, err := cm.CandidateVolatileState.AddVoteFrom(
			fromPeer, rpcRequestVoteReply.VoteGranted,
		)
		if err != nil {
			return err
		}
		if haveQuorum {
			cm.logger.Println("[raft] have quorum - won election!")
			err = cm.becomeLeader()
			if err != nil {
				return err
			}
		}
	} // else: Ignore - not a candidate

	return nil
}
