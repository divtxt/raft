package raft

import (
	"reflect"
	"testing"
)

// #RFS-C2: If votes received from majority of servers: become leader
// #5.2-p3s1: A candidate wins an election if it receives votes from a
// majority of the servers in the full cluster for the same term.
// #RFS-L1a: Upon election: send initial empty AppendEntries RPCs (heartbeat)
// to each server;
func TestCM_RpcRVR_Candidate_CandidateWinsElectionIfItReceivesMajorityOfVotes(t *testing.T) {
	mcm, mrs := testSetupMCM_Candidate_Figure7LeaderLine(t)
	serverTerm := mcm.pcm.persistentState.GetCurrentTerm()
	mcm.pcm.setCommitIndex(3)
	sentRpc := &RpcRequestVote{serverTerm, 0, 0}

	// s2 grants vote - should stay as candidate
	mcm.pcm.rpcReply_RpcRequestVoteReply(
		"s2",
		sentRpc,
		&RpcRequestVoteReply{serverTerm, true},
	)
	if mcm.pcm.getServerState() != CANDIDATE {
		t.Fatal()
	}

	// s3 denies vote - should stay as candidate
	mcm.pcm.rpcReply_RpcRequestVoteReply(
		"s3",
		sentRpc,
		&RpcRequestVoteReply{serverTerm, false},
	)
	if mcm.pcm.getServerState() != CANDIDATE {
		t.Fatal()
	}

	// s4 grants vote - should become leader
	mcm.pcm.rpcReply_RpcRequestVoteReply(
		"s4",
		sentRpc,
		&RpcRequestVoteReply{serverTerm, true},
	)
	testIsLeaderWithTermAndSentEmptyAppendEntries(t, mcm, mrs, serverTerm)

	// s5 grants vote - should stay leader
	mcm.pcm.rpcReply_RpcRequestVoteReply(
		"s5",
		sentRpc,
		&RpcRequestVoteReply{serverTerm, true},
	)
	if mcm.pcm.getServerState() != LEADER {
		t.Fatal()
	}
	if mcm.pcm.persistentState.GetCurrentTerm() != serverTerm {
		t.Fatal()
	}
}

func testIsLeaderWithTermAndSentEmptyAppendEntries(
	t *testing.T,
	mcm *managedConsensusModule,
	mrs *mockRpcSender,
	serverTerm TermNo,
) {
	if mcm.pcm.getServerState() != LEADER {
		t.Fatal()
	}
	if mcm.pcm.persistentState.GetCurrentTerm() != serverTerm {
		t.Fatal()
	}

	// leader state is fresh
	expectedNextIndex := map[ServerId]LogIndex{"s2": 11, "s3": 11, "s4": 11, "s5": 11}
	if !reflect.DeepEqual(mcm.pcm.leaderVolatileState.nextIndex, expectedNextIndex) {
		t.Fatal()
	}
	expectedMatchIndex := map[ServerId]LogIndex{"s2": 0, "s3": 0, "s4": 0, "s5": 0}
	if !reflect.DeepEqual(mcm.pcm.leaderVolatileState.matchIndex, expectedMatchIndex) {
		t.Fatal()
	}

	// leader setup
	lastLogIndex, lastLogTerm := GetIndexAndTermOfLastEntry(mcm.pcm.log)
	expectedRpc := &RpcAppendEntries{
		serverTerm,
		lastLogIndex,
		lastLogTerm,
		[]LogEntry{},
		mcm.pcm.getCommitIndex(),
	}
	expectedRpcs := []mockSentRpc{
		{"s2", expectedRpc},
		{"s3", expectedRpc},
		{"s4", expectedRpc},
		{"s5", expectedRpc},
	}
	mrs.checkSentRpcs(t, expectedRpcs)
}

// #RFS-C4: If election timeout elapses: start new election
// #5.2-p5s1: The third possible outcome is that a candidate neither
// wins nor loses the election; ... votes could be split so that no
// candidate obtains a majority.
// #5.2-p5s2: When this happens, each candidate will time out and
// start a new election by incrementing its term and initiating
// another round of RequestVote RPCs.
func TestCM_RpcRVR_Candidate_StartNewElectionOnElectionTimeout(t *testing.T) {
	mcm, mrs := testSetupMCM_Candidate_Figure7LeaderLine(t)
	serverTerm := mcm.pcm.persistentState.GetCurrentTerm()
	sentRpc := &RpcRequestVote{serverTerm, 0, 0}

	// s2 grants vote - should stay as candidate
	mcm.pcm.rpcReply_RpcRequestVoteReply(
		"s2",
		sentRpc,
		&RpcRequestVoteReply{serverTerm, true},
	)
	if mcm.pcm.getServerState() != CANDIDATE {
		t.Fatal()
	}

	// s3 denies vote - should stay as candidate
	mcm.pcm.rpcReply_RpcRequestVoteReply(
		"s3",
		sentRpc,
		&RpcRequestVoteReply{serverTerm, false},
	)
	if mcm.pcm.getServerState() != CANDIDATE {
		t.Fatal()
	}

	// no more votes - election timeout causes a new election
	testCM_FollowerOrCandidate_StartsElectionOnElectionTimeout_Part2(t, mcm, mrs, serverTerm+1)
}

// Extra: follower or leader ignores vote
func TestCM_RpcRVR_FollowerOrLeader_Ignores(t *testing.T) {
	f := func(setup func(t *testing.T) (mcm *managedConsensusModule, mrs *mockRpcSender)) {
		mcm, mrs := setup(t)
		serverTerm := mcm.pcm.persistentState.GetCurrentTerm()
		sentRpc := &RpcRequestVote{serverTerm, 0, 0}
		beforeState := mcm.pcm.getServerState()

		// s2 grants vote - ignore
		mcm.pcm.rpcReply_RpcRequestVoteReply(
			"s2",
			sentRpc,
			&RpcRequestVoteReply{serverTerm, true},
		)
		if mcm.pcm.getServerState() != beforeState {
			t.Fatal()
		}
		mrs.checkSentRpcs(t, []mockSentRpc{})

		// s3 denies vote - ignore
		mcm.pcm.rpcReply_RpcRequestVoteReply(
			"s3",
			sentRpc,
			&RpcRequestVoteReply{serverTerm, false},
		)
		if mcm.pcm.getServerState() != beforeState {
			t.Fatal()
		}
		mrs.checkSentRpcs(t, []mockSentRpc{})

		// s4 grants vote - ignore
		mcm.pcm.rpcReply_RpcRequestVoteReply(
			"s4",
			sentRpc,
			&RpcRequestVoteReply{serverTerm, true},
		)
		if mcm.pcm.getServerState() != beforeState {
			t.Fatal()
		}
		mrs.checkSentRpcs(t, []mockSentRpc{})
	}

	f(testSetupMCM_Follower_Figure7LeaderLine)
	f(testSetupMCM_Leader_Figure7LeaderLine)
}

// Extra: ignore replies for previous term rpc
// #RFS-A2: If RPC request or response contains term T > currentTerm:
// set currentTerm = T, convert to follower (#5.1)
// #5.1-p3s4: ...; if one server's current term is smaller than the
// other's, then it updates its current term to the larger value.
// #5.1-p3s5: If a candidate or leader discovers that its term is out of
// date, it immediately reverts to follower state.
func TestCM_RpcRVR_All_RpcTermMismatches(t *testing.T) {
	f := func(
		setup func(t *testing.T) (mcm *managedConsensusModule, mrs *mockRpcSender),
		sendMajorityVote bool,
	) {
		mcm, mrs := setup(t)
		serverTerm := mcm.pcm.persistentState.GetCurrentTerm()
		mcm.pcm.setCommitIndex(2)
		sentRpc := &RpcRequestVote{serverTerm, 0, 0}
		beforeState := mcm.pcm.getServerState()

		// s2 grants vote - should stay as candidate
		mcm.pcm.rpcReply_RpcRequestVoteReply(
			"s2",
			sentRpc,
			&RpcRequestVoteReply{serverTerm, true},
		)
		if mcm.pcm.getServerState() != beforeState {
			t.Fatal()
		}

		// s3 grants vote for previous term election - ignore and stay as candidate
		mcm.pcm.rpcReply_RpcRequestVoteReply(
			"s3",
			&RpcRequestVote{serverTerm - 1, 0, 0},
			&RpcRequestVoteReply{serverTerm - 1, true},
		)
		if mcm.pcm.getServerState() != beforeState {
			t.Fatal()
		}

		if sendMajorityVote {
			// s3 grants vote for this election - become leader only if candidate
			mcm.pcm.rpcReply_RpcRequestVoteReply(
				"s3",
				sentRpc,
				&RpcRequestVoteReply{serverTerm, true},
			)
			if beforeState == CANDIDATE {
				testIsLeaderWithTermAndSentEmptyAppendEntries(t, mcm, mrs, serverTerm)
			} else {
				if mcm.pcm.getServerState() != beforeState {
					t.Fatal()
				}
			}
		}

		// s4 denies vote for this election indicating a newer term - increase term
		// and become follower
		mcm.pcm.rpcReply_RpcRequestVoteReply(
			"s4",
			sentRpc,
			&RpcRequestVoteReply{serverTerm + 1, false},
		)
		if mcm.pcm.getServerState() != FOLLOWER {
			t.Fatal()
		}
		if mcm.pcm.persistentState.GetCurrentTerm() != serverTerm+1 {
			t.Fatal()
		}
		if mcm.pcm.persistentState.GetVotedFor() != "" {
			t.Fatal()
		}
	}

	f(testSetupMCM_Candidate_Figure7LeaderLine, true)
	f(testSetupMCM_Candidate_Figure7LeaderLine, false)

	f(testSetupMCM_Follower_Figure7LeaderLine, true)
	f(testSetupMCM_Follower_Figure7LeaderLine, false)

	f(testSetupMCM_Leader_Figure7LeaderLine, true)
	f(testSetupMCM_Leader_Figure7LeaderLine, false)
}
