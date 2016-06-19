package raft

import (
	"testing"
)

// 1. Reply false if term < currentTerm (#5.1)
// Note: test based on Figure 7; server is leader line; peer is case (a)
func TestCM_RpcRV_TermLessThanCurrentTerm(t *testing.T) {
	f := func(setup func(t *testing.T) (mcm *managedConsensusModule, mrs *mockRpcSender)) {
		mcm, _ := setup(t)
		serverTerm := mcm.pcm.raftPersistentState.GetCurrentTerm()
		electionTimeoutTime1 := mcm.pcm.electionTimeoutTracker.electionTimeoutTime

		requestVote := &RpcRequestVote{7, 9, 6}

		reply, err := mcm.rpc_RpcRequestVote("s2", requestVote)
		if err != nil {
			t.Fatal(err)
		}

		expectedRpc := RpcRequestVoteReply{serverTerm, false}
		if *reply != expectedRpc {
			t.Fatal(reply)
		}

		// #RFS-F2: (paraphrasing) not granting vote should allow election timeout
		if mcm.pcm.electionTimeoutTracker.electionTimeoutTime != electionTimeoutTime1 {
			t.Fatal()
		}
	}

	f(testSetupMCM_Follower_Figure7LeaderLine)
	f(testSetupMCM_Candidate_Figure7LeaderLine)
	f(testSetupMCM_Leader_Figure7LeaderLine)
}

// 2. If votedFor is null or candidateId, and candidate's log is at least as
// up-to-date as receiver's log, grant vote (#5.2, #5.4)
// #5.4.1-p3s1: Raft determines which of two logs is more up-to-date by
// comparing the index and term of the last entries in the logs.
// #5.4.1-p3s2: If the logs have last entries with different terms, then
// the log with the later term is more up-to-date.
// #5.4.1-p3s3: If the logs end with the same term, then whichever log is
// longer is more up-to-date.
// Note: test based on Figure 7; server is leader line; peer is case (d)
func TestCM_RpcRV_SameTerm_All_VotedForOther(t *testing.T) {
	f := func(setup func(t *testing.T) (mcm *managedConsensusModule, mrs *mockRpcSender)) {
		mcm, _ := setup(t)
		serverTerm := mcm.pcm.raftPersistentState.GetCurrentTerm()
		beforeState := mcm.pcm.getServerState()
		electionTimeoutTime1 := mcm.pcm.electionTimeoutTracker.electionTimeoutTime

		// sanity check
		votedFor := mcm.pcm.raftPersistentState.GetVotedFor()
		if votedFor != "s1" && votedFor != "s2" {
			t.Fatal(votedFor)
		}

		requestVote := &RpcRequestVote{serverTerm, 12, 7}

		reply, err := mcm.rpc_RpcRequestVote("s3", requestVote)
		if err != nil {
			t.Fatal(err)
		}

		if mcm.pcm.getServerState() != beforeState {
			t.Fatal()
		}
		if mcm.pcm.raftPersistentState.GetCurrentTerm() != serverTerm {
			t.Fatal()
		}

		expectedRpc := RpcRequestVoteReply{serverTerm, false}
		if *reply != expectedRpc {
			t.Fatal(reply)
		}

		// #RFS-F2: (paraphrasing) not granting vote should allow election timeout
		if mcm.pcm.electionTimeoutTracker.electionTimeoutTime != electionTimeoutTime1 {
			t.Fatal()
		}
	}

	// Follower that voted for s2
	f(testSetupMCM_FollowerThatVotedForS2_Figure7LeaderLine)

	// Candidate - has to have voted for itself
	f(testSetupMCM_Candidate_Figure7LeaderLine)

	// Leader - has to have voted for itself
	f(testSetupMCM_Leader_Figure7LeaderLine)
}

// Note: test based on Figure 7; server is leader line; peer is case (d)
func TestCM_RpcRV_SameTerm_Follower_NullVoteOrSameVote(t *testing.T) {
	f := func(setup func(t *testing.T) (mcm *managedConsensusModule, mrs *mockRpcSender)) {
		mcm, _ := setup(t)
		serverTerm := mcm.pcm.raftPersistentState.GetCurrentTerm()
		electionTimeoutTime1 := mcm.pcm.electionTimeoutTracker.electionTimeoutTime

		// sanity check
		votedFor := mcm.pcm.raftPersistentState.GetVotedFor()
		if votedFor != "" && votedFor != "s2" {
			t.Fatal(votedFor)
		}

		requestVote := &RpcRequestVote{serverTerm, 12, 7}

		reply, err := mcm.rpc_RpcRequestVote("s2", requestVote)
		if err != nil {
			t.Fatal(err)
		}

		expectedRpc := RpcRequestVoteReply{serverTerm, true}
		if *reply != expectedRpc {
			t.Fatal(reply)
		}

		// #RFS-F2: (paraphrasing) granting vote should prevent election timeout
		if mcm.pcm.electionTimeoutTracker.electionTimeoutTime == electionTimeoutTime1 {
			t.Fatal()
		}
	}

	// Follower that did not vote
	f(testSetupMCM_Follower_Figure7LeaderLine)
	// Follower that voted for s2
	f(testSetupMCM_FollowerThatVotedForS2_Figure7LeaderLine)

	// Candidate -  invalid case - has to have voted for itself

	// Leader -  invalid case - has to have voted for itself
}

// Note: test based on Figure 7; server is leader line; peer is case (d)
func TestCM_RpcRV_SameTerm_CandidateOrLeader_SelfVote(t *testing.T) {
	f := func(setup func(t *testing.T) (mcm *managedConsensusModule, mrs *mockRpcSender)) {
		mcm, _ := setup(t)
		serverTerm := mcm.pcm.raftPersistentState.GetCurrentTerm()
		electionTimeoutTime1 := mcm.pcm.electionTimeoutTracker.electionTimeoutTime

		// sanity check
		votedFor := mcm.pcm.raftPersistentState.GetVotedFor()
		if votedFor != "s1" {
			t.Fatal(votedFor)
		}

		requestVote := &RpcRequestVote{serverTerm, 12, 7}

		reply, err := mcm.rpc_RpcRequestVote("s2", requestVote)
		if err != nil {
			t.Fatal(err)
		}

		expectedRpc := RpcRequestVoteReply{serverTerm, false}
		if *reply != expectedRpc {
			t.Fatal(reply)
		}

		// #RFS-F2: (paraphrasing) not granting vote should allow election timeout
		if mcm.pcm.electionTimeoutTracker.electionTimeoutTime != electionTimeoutTime1 {
			t.Fatal()
		}
	}

	// Follower - invalid case - cannot have voted for itself

	// Candidate - has to have voted for itself
	f(testSetupMCM_Candidate_Figure7LeaderLine)

	// Leader - has to have voted for itself
	f(testSetupMCM_Leader_Figure7LeaderLine)
}

func testSetupMCM_FollowerThatVotedForS2_Figure7LeaderLine(
	t *testing.T,
) (*managedConsensusModule, *mockRpcSender) {
	mcm, mrs := testSetupMCM_Follower_WithTerms(t, makeLogTerms_Figure7LeaderLine())

	// sanity check
	if mcm.pcm.raftPersistentState.GetVotedFor() != "" {
		t.Fatal()
	}
	// pretend server voted
	err := mcm.pcm.raftPersistentState.SetVotedFor("s2")
	if err != nil {
		t.Fatal(err)
	}

	return mcm, mrs
}

// Note: test based on Figure 7; server is leader line; sender is case (b)
func TestCM_RpcRV_NewerTerm_SenderHas_OlderTerm_SmallerIndex(t *testing.T) {
	testCM_RpcRV_NewerTerm_SenderHasGivenLastEntryIndexAndTerm(t, 4, 4, false)
}

// Note: test based on Figure 7; server is leader line; sender is extension of
// case (e):
// 1, 1, 1, 4, 4, 4, 4, 4, 4, 4
func TestCM_RpcRV_NewerTerm_SenderHas_OlderTerm_SameIndex(t *testing.T) {
	testCM_RpcRV_NewerTerm_SenderHasGivenLastEntryIndexAndTerm(t, 10, 4, false)
}

// Note: test based on Figure 7; server is leader line; sender is case (f)
func TestCM_RpcRV_NewerTerm_SenderHas_OlderTerm_LargerIndex(t *testing.T) {
	testCM_RpcRV_NewerTerm_SenderHasGivenLastEntryIndexAndTerm(t, 11, 3, false)
}

// Note: test based on Figure 7; server is leader line; sender is variant of
// case (a):
// 1, 1, 1, 4, 4, 5, 5, 6, 7
func TestCM_RpcRV_NewerTerm_SenderHas_NewerTerm_SmallerIndex(t *testing.T) {
	testCM_RpcRV_NewerTerm_SenderHasGivenLastEntryIndexAndTerm(t, 9, 7, true)
}

// Note: test based on Figure 7; server is leader line; sender is extention of
// case (a):
// 1, 1, 1, 4, 4, 5, 5, 6, 6, 7
func TestCM_RpcRV_NewerTerm_SenderHas_NewerTerm_SameIndex(t *testing.T) {
	// this case can go either way!
	testCM_RpcRV_NewerTerm_SenderHasGivenLastEntryIndexAndTerm(t, 10, 7, true)
}

// Note: test based on Figure 7; server is leader line; sender is case (d)
func TestCM_RpcRV_NewerTerm_SenderHas_NewerTerm_LargerIndex(t *testing.T) {
	testCM_RpcRV_NewerTerm_SenderHasGivenLastEntryIndexAndTerm(t, 12, 7, true)
}

// Note: test based on Figure 7; server is leader line; sender is case (a)
func TestCM_RpcRV_NewerTerm_SenderHas_SameTerm_SmallerIndex(t *testing.T) {
	testCM_RpcRV_NewerTerm_SenderHasGivenLastEntryIndexAndTerm(t, 9, 6, false)
}

// Note: test based on Figure 7; server is leader line; sender is same
func TestCM_RpcRV_NewerTerm_SenderHas_SameTerm_SameIndex(t *testing.T) {
	testCM_RpcRV_NewerTerm_SenderHasGivenLastEntryIndexAndTerm(t, 10, 6, true)
}

// Note: test based on Figure 7; server is leader line; sender is case (c)
func TestCM_RpcRV_NewerTerm_SenderHas_SameTerm_LargerIndex(t *testing.T) {
	testCM_RpcRV_NewerTerm_SenderHasGivenLastEntryIndexAndTerm(t, 11, 6, true)
}

func testCM_RpcRV_NewerTerm_SenderHasGivenLastEntryIndexAndTerm(
	t *testing.T,
	senderLastEntryIndex LogIndex,
	senderLastEntryTerm TermNo,
	expectedVote bool,
) {
	f := func(setup func(t *testing.T) (mcm *managedConsensusModule, mrs *mockRpcSender)) {
		mcm, _ := setup(t)
		serverTerm := mcm.pcm.raftPersistentState.GetCurrentTerm()
		electionTimeoutTime1 := mcm.pcm.electionTimeoutTracker.electionTimeoutTime

		// sanity checks
		if serverTerm != 8 {
			t.Fatal(serverTerm)
		}
		votedFor := mcm.pcm.raftPersistentState.GetVotedFor()
		if votedFor != "" && votedFor != "s1" {
			t.Fatal(votedFor)
		}

		requestVote := &RpcRequestVote{10, senderLastEntryIndex, senderLastEntryTerm}

		reply, err := mcm.rpc_RpcRequestVote("s5", requestVote)
		if err != nil {
			t.Fatal(err)
		}

		// #RFS-A2: If RPC request or response contains term T > currentTerm:
		// set currentTerm = T, convert to follower (#5.1)
		// #5.1-p3s4: ...; if one server's current term is smaller than the
		// other's, then it updates its current term to the larger value.
		// #5.1-p3s5: If a candidate or leader discovers that its term is out of
		// date, it immediately reverts to follower state.
		if mcm.pcm.getServerState() != FOLLOWER {
			t.Fatal()
		}
		if mcm.pcm.raftPersistentState.GetCurrentTerm() != 10 {
			t.Fatal(mcm.pcm.raftPersistentState.GetCurrentTerm())
		}
		var expectedVotedFor ServerId = ""
		if expectedVote {
			expectedVotedFor = "s5"
		}
		actualVotedFor := mcm.pcm.raftPersistentState.GetVotedFor()
		if actualVotedFor != expectedVotedFor {
			t.Fatal(actualVotedFor)
		}

		expectedRpc := RpcRequestVoteReply{10, expectedVote}
		if *reply != expectedRpc {
			t.Fatal(reply)
		}

		if expectedVote {
			// #RFS-F2: (paraphrasing) granting vote should prevent election timeout
			if mcm.pcm.electionTimeoutTracker.electionTimeoutTime == electionTimeoutTime1 {
				t.Fatal()
			}
		} else {
			// #RFS-F2: (paraphrasing) not granting vote should allow election timeout
			if mcm.pcm.electionTimeoutTracker.electionTimeoutTime != electionTimeoutTime1 {
				t.Fatal()
			}
		}
	}

	f(testSetupMCM_FollowerTerm8_Figure7LeaderLine)
	f(testSetupMCM_Candidate_Figure7LeaderLine)
	f(testSetupMCM_Leader_Figure7LeaderLine)
}

func testSetupMCM_FollowerTerm8_Figure7LeaderLine(t *testing.T) (*managedConsensusModule, *mockRpcSender) {
	mcm, mrs := testSetupMCM_Follower_WithTerms(t, makeLogTerms_Figure7LeaderLine())
	serverTerm := mcm.pcm.raftPersistentState.GetCurrentTerm()

	// sanity check
	if serverTerm != 7 {
		t.Fatal(serverTerm)
	}
	// pretend server was pushed to term 8
	err := mcm.pcm.raftPersistentState.SetCurrentTerm(8)
	if err != nil {
		t.Fatal(err)
	}

	return mcm, mrs
}
