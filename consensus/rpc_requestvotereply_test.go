package consensus

import (
	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/testhelpers"
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
	serverTerm := mcm.pcm.RaftPersistentState.GetCurrentTerm()
	err := mcm.pcm.setCommitIndex(3)
	if err != nil {
		t.Fatal(err)
	}
	sentRpc := &RpcRequestVote{serverTerm, 0, 0}

	// s2 grants vote - should stay as candidate
	err = mcm.pcm.RpcReply_RpcRequestVoteReply(
		"s2",
		sentRpc,
		&RpcRequestVoteReply{serverTerm, true},
	)
	if err != nil {
		t.Fatal(err)
	}
	if mcm.pcm.GetServerState() != CANDIDATE {
		t.Fatal()
	}

	// s3 denies vote - should stay as candidate
	err = mcm.pcm.RpcReply_RpcRequestVoteReply(
		"s3",
		sentRpc,
		&RpcRequestVoteReply{serverTerm, false},
	)
	if err != nil {
		t.Fatal(err)
	}
	if mcm.pcm.GetServerState() != CANDIDATE {
		t.Fatal()
	}

	// s4 grants vote - should become leader
	err = mcm.pcm.RpcReply_RpcRequestVoteReply(
		"s4",
		sentRpc,
		&RpcRequestVoteReply{serverTerm, true},
	)
	if err != nil {
		t.Fatal(err)
	}
	testIsLeaderWithTermAndSentEmptyAppendEntries(t, mcm, mrs, serverTerm)

	// s5 grants vote - should stay leader
	err = mcm.pcm.RpcReply_RpcRequestVoteReply(
		"s5",
		sentRpc,
		&RpcRequestVoteReply{serverTerm, true},
	)
	if err != nil {
		t.Fatal(err)
	}
	if mcm.pcm.GetServerState() != LEADER {
		t.Fatal()
	}
	if mcm.pcm.RaftPersistentState.GetCurrentTerm() != serverTerm {
		t.Fatal()
	}
}

func testIsLeaderWithTermAndSentEmptyAppendEntries(
	t *testing.T,
	mcm *managedConsensusModule,
	mrs *testhelpers.MockRpcSender,
	serverTerm TermNo,
) {
	if mcm.pcm.GetServerState() != LEADER {
		t.Fatal()
	}
	if mcm.pcm.RaftPersistentState.GetCurrentTerm() != serverTerm {
		t.Fatal()
	}

	// leader state is fresh
	expectedNextIndex := map[ServerId]LogIndex{"s2": 11, "s3": 11, "s4": 11, "s5": 11}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.NextIndex, expectedNextIndex) {
		t.Fatal()
	}
	expectedMatchIndex := map[ServerId]LogIndex{"s2": 0, "s3": 0, "s4": 0, "s5": 0}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.MatchIndex, expectedMatchIndex) {
		t.Fatal()
	}

	// leader setup
	lastLogIndex, lastLogTerm, err := GetIndexAndTermOfLastEntry(mcm.pcm.LogRO)
	if err != nil {
		t.Fatal(err)
	}
	expectedRpc := &RpcAppendEntries{
		serverTerm,
		lastLogIndex,
		lastLogTerm,
		[]LogEntry{},
		mcm.pcm.GetCommitIndex(),
	}
	expectedRpcs := []testhelpers.MockSentRpc{
		{"s2", expectedRpc},
		{"s3", expectedRpc},
		{"s4", expectedRpc},
		{"s5", expectedRpc},
	}
	mrs.CheckSentRpcs(t, expectedRpcs)
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
	serverTerm := mcm.pcm.RaftPersistentState.GetCurrentTerm()
	sentRpc := &RpcRequestVote{serverTerm, 0, 0}

	// s2 grants vote - should stay as candidate
	err := mcm.pcm.RpcReply_RpcRequestVoteReply(
		"s2",
		sentRpc,
		&RpcRequestVoteReply{serverTerm, true},
	)
	if err != nil {
		t.Fatal(err)
	}
	if mcm.pcm.GetServerState() != CANDIDATE {
		t.Fatal()
	}

	// s3 denies vote - should stay as candidate
	err = mcm.pcm.RpcReply_RpcRequestVoteReply(
		"s3",
		sentRpc,
		&RpcRequestVoteReply{serverTerm, false},
	)
	if err != nil {
		t.Fatal(err)
	}
	if mcm.pcm.GetServerState() != CANDIDATE {
		t.Fatal()
	}

	// no more votes - election timeout causes a new election
	testCM_FollowerOrCandidate_StartsElectionOnElectionTimeout_Part2(t, mcm, mrs, serverTerm+1)
}

// Extra: follower or leader ignores vote
func TestCM_RpcRVR_FollowerOrLeader_Ignores(t *testing.T) {
	f := func(setup func(t *testing.T) (mcm *managedConsensusModule, mrs *testhelpers.MockRpcSender)) {
		mcm, mrs := setup(t)
		serverTerm := mcm.pcm.RaftPersistentState.GetCurrentTerm()
		sentRpc := &RpcRequestVote{serverTerm, 0, 0}
		beforeState := mcm.pcm.GetServerState()

		// s2 grants vote - ignore
		err := mcm.pcm.RpcReply_RpcRequestVoteReply(
			"s2",
			sentRpc,
			&RpcRequestVoteReply{serverTerm, true},
		)
		if err != nil {
			t.Fatal(err)
		}
		if mcm.pcm.GetServerState() != beforeState {
			t.Fatal()
		}
		mrs.CheckSentRpcs(t, []testhelpers.MockSentRpc{})

		// s3 denies vote - ignore
		err = mcm.pcm.RpcReply_RpcRequestVoteReply(
			"s3",
			sentRpc,
			&RpcRequestVoteReply{serverTerm, false},
		)
		if err != nil {
			t.Fatal(err)
		}
		if mcm.pcm.GetServerState() != beforeState {
			t.Fatal()
		}
		mrs.CheckSentRpcs(t, []testhelpers.MockSentRpc{})

		// s4 grants vote - ignore
		err = mcm.pcm.RpcReply_RpcRequestVoteReply(
			"s4",
			sentRpc,
			&RpcRequestVoteReply{serverTerm, true},
		)
		if err != nil {
			t.Fatal(err)
		}
		if mcm.pcm.GetServerState() != beforeState {
			t.Fatal()
		}
		mrs.CheckSentRpcs(t, []testhelpers.MockSentRpc{})
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
		setup func(t *testing.T) (mcm *managedConsensusModule, mrs *testhelpers.MockRpcSender),
		sendMajorityVote bool,
	) {
		mcm, mrs := setup(t)
		serverTerm := mcm.pcm.RaftPersistentState.GetCurrentTerm()
		err := mcm.pcm.setCommitIndex(2)
		if err != nil {
			t.Fatal(err)
		}
		sentRpc := &RpcRequestVote{serverTerm, 0, 0}
		beforeState := mcm.pcm.GetServerState()

		// s2 grants vote - should stay as candidate
		err = mcm.pcm.RpcReply_RpcRequestVoteReply(
			"s2",
			sentRpc,
			&RpcRequestVoteReply{serverTerm, true},
		)
		if err != nil {
			t.Fatal(err)
		}
		if mcm.pcm.GetServerState() != beforeState {
			t.Fatal()
		}

		// s3 grants vote for previous term election - ignore and stay as candidate
		err = mcm.pcm.RpcReply_RpcRequestVoteReply(
			"s3",
			&RpcRequestVote{serverTerm - 1, 0, 0},
			&RpcRequestVoteReply{serverTerm - 1, true},
		)
		if err != nil {
			t.Fatal(err)
		}
		if mcm.pcm.GetServerState() != beforeState {
			t.Fatal()
		}

		if sendMajorityVote {
			// s3 grants vote for this election - become leader only if candidate
			err = mcm.pcm.RpcReply_RpcRequestVoteReply(
				"s3",
				sentRpc,
				&RpcRequestVoteReply{serverTerm, true},
			)
			if err != nil {
				t.Fatal(err)
			}
			if beforeState == CANDIDATE {
				testIsLeaderWithTermAndSentEmptyAppendEntries(t, mcm, mrs, serverTerm)
			} else {
				if mcm.pcm.GetServerState() != beforeState {
					t.Fatal()
				}
			}
		}

		// s4 denies vote for this election indicating a newer term - increase term
		// and become follower
		err = mcm.pcm.RpcReply_RpcRequestVoteReply(
			"s4",
			sentRpc,
			&RpcRequestVoteReply{serverTerm + 1, false},
		)
		if err != nil {
			t.Fatal(err)
		}
		if mcm.pcm.GetServerState() != FOLLOWER {
			t.Fatal()
		}
		if mcm.pcm.RaftPersistentState.GetCurrentTerm() != serverTerm+1 {
			t.Fatal()
		}
		if mcm.pcm.RaftPersistentState.GetVotedFor() != "" {
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
