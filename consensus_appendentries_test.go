package raft

import (
	"testing"
)

func makeAEWithTerm(term TermNo) *RpcAppendEntries {
	return &RpcAppendEntries{term, 0, 0, nil, 0}
}

func makeAEWithTermAndPrevLogDetails(term TermNo, prevli LogIndex, prevterm TermNo) *RpcAppendEntries {
	return &RpcAppendEntries{term, prevli, prevterm, nil, 0}
}

// 1. Reply false if term < currentTerm (#5.1)
func TestCM_RpcAE_LeaderTermLessThanCurrentTerm(t *testing.T) {
	f := func(
		setup func(t *testing.T) (mcm *managedConsensusModule, mrs *mockRpcSender),
	) (*managedConsensusModule, *mockRpcSender) {
		mcm, mrs := setup(t)
		serverTerm := mcm.pcm.persistentState.GetCurrentTerm()
		electionTimeoutTime1 := mcm.pcm.electionTimeoutTracker.electionTimeoutTime

		appendEntries := makeAEWithTerm(serverTerm - 1)

		reply := mcm.rpc_RpcAppendEntries("s2", appendEntries)

		expectedRpc := RpcAppendEntriesReply{serverTerm, false}
		if *reply != expectedRpc {
			t.Fatal(reply)
		}

		// #RFS-F2: (paraphrasing) AppendEntries RPC not from current leader should
		// allow election timeout
		if mcm.pcm.electionTimeoutTracker.electionTimeoutTime != electionTimeoutTime1 {
			t.Fatal()
		}

		return mcm, mrs
	}

	// Follower
	f(testSetupMCM_Follower_Figure7LeaderLine)

	// Candidate
	{
		mcm, _ := f(testSetupMCM_Candidate_Figure7LeaderLine)

		// #5.2-p4s1: While waiting for votes, a candidate may receive an AppendEntries
		// RPC from another server claiming to be leader.
		// #5.2-p4s3: If the term in the RPC is smaller than the candidate’s current
		// term, then the candidate rejects the RPC and continues in candidate state.
		if mcm.pcm.getServerState() != CANDIDATE {
			t.Fatal()
		}
	}

	// Leader
	{
		mcm, _ := f(testSetupMCM_Leader_Figure7LeaderLine)

		// Assumed: leader ignores a leader from an older term
		if mcm.pcm.getServerState() != LEADER {
			t.Fatal()
		}
	}
}

// 2. Reply false if log doesn't contain an entry at prevLogIndex whose term
//      matches prevLogTerm (#5.3)
// Note: the above language is slightly ambiguous but I'm assuming that this
// step refers strictly to the follower's log not having any entry at
// prevLogIndex since step 3 covers the alternate conflicting entry case.
// Note: this test based on Figure 7, server (b)
func TestCM_RpcAE_NoMatchingLogEntry(t *testing.T) {
	f := func(
		setup func(*testing.T, []TermNo) (*managedConsensusModule, *mockRpcSender),
		senderTermIsNewer bool,
	) (*managedConsensusModule, *mockRpcSender) {
		mcm, mrs := setup(t, []TermNo{1, 1, 1, 4})
		serverTerm := mcm.pcm.persistentState.GetCurrentTerm()
		electionTimeoutTime1 := mcm.pcm.electionTimeoutTracker.electionTimeoutTime

		senderTerm := serverTerm
		if senderTermIsNewer {
			senderTerm += 1
		}

		appendEntries := makeAEWithTermAndPrevLogDetails(senderTerm, 10, 6)

		reply := mcm.rpc_RpcAppendEntries("s3", appendEntries)

		expectedRpc := RpcAppendEntriesReply{senderTerm, false}
		if *reply != expectedRpc {
			t.Fatal(reply)
		}

		if senderTermIsNewer {
			// #RFS-A2: If RPC request or response contains term T > currentTerm:
			// set currentTerm = T, convert to follower (#5.1)

			// #RFS-C3: If AppendEntries RPC received from new leader:
			// convert to follower
			// #5.1-p3s4: ...; if one server's current term is smaller than the
			// other's, then it updates its current term to the larger value.
			// #5.1-p3s5: If a candidate or leader discovers that its term is out of
			// date, it immediately reverts to follower state.
			// #5.2-p4s1: While waiting for votes, a candidate may receive an
			// AppendEntries RPC from another server claiming to be leader.
			// #5.2-p4s2: If the leader’s term (included in its RPC) is at least as
			// large as the candidate’s current term, then the candidate recognizes
			// the leader as legitimate and returns to follower state.
			if mcm.pcm.getServerState() != FOLLOWER {
				t.Fatal()
			}
		}
		if mcm.pcm.persistentState.GetCurrentTerm() != senderTerm {
			t.Fatal()
		}

		// #RFS-F2: (paraphrasing) AppendEntries RPC from current leader should
		// prevent election timeout
		if mcm.pcm.electionTimeoutTracker.electionTimeoutTime == electionTimeoutTime1 {
			t.Fatal()
		}

		return mcm, mrs
	}

	// Follower
	f(testSetupMCM_Follower_WithTerms, true)
	f(testSetupMCM_Follower_WithTerms, false)

	// Candidate
	f(testSetupMCM_Candidate_WithTerms, true)
	f(testSetupMCM_Candidate_WithTerms, false)

	// Leader
	f(testSetupMCM_Leader_WithTerms, true)
	// Extra: raft violation - two leaders with same term
	test_ExpectPanic(
		t,
		func() {
			f(testSetupMCM_Leader_WithTerms, false)
		},
		"FATAL: two leaders with same term - got AppendEntries from: s3 with term: 8",
	)
}

// 3. If an existing entry conflicts with a new one (same index
// but different terms), delete the existing entry and all that
// follow it (#5.3)
// 4. Append any new entries not already in the log
// 5. If leaderCommit > commitIndex, set commitIndex = min(leaderCommit,
// index of last new entry)
// Note: this is not specified, but I'm assuming that the RPC reply should
// have success set to true.
// Note: this test case based on Figure 7, case (e) in the Raft paper but adds
// some extra entries to also test step 3
func TestCM_RpcAE_AppendNewEntries(t *testing.T) {
	f := func(
		setup func(t *testing.T, terms []TermNo) (mcm *managedConsensusModule, mrs *mockRpcSender),
		senderTermIsNewer bool,
	) {
		mcm, _ := setup(
			t,
			[]TermNo{1, 1, 1, 4, 4, 4, 4, 4, 4, 4, 4},
		)
		mcm.pcm.setCommitIndex(3)

		serverTerm := mcm.pcm.persistentState.GetCurrentTerm()
		electionTimeoutTime1 := mcm.pcm.electionTimeoutTracker.electionTimeoutTime

		senderTerm := serverTerm
		if senderTermIsNewer {
			senderTerm += 1
		}

		if !testCommandEquals(testHelper_GetLogEntryAtIndex(mcm.pcm.log, 6).Command, "c6") {
			t.Error()
		}

		sentLogEntries := []LogEntry{
			{5, Command("c6'")},
			{5, Command("c7'")},
			{6, Command("c8'")},
		}

		appendEntries := &RpcAppendEntries{senderTerm, 5, 4, sentLogEntries, 7}

		reply := mcm.rpc_RpcAppendEntries("s4", appendEntries)

		expectedRpc := RpcAppendEntriesReply{senderTerm, true}
		if *reply != expectedRpc {
			t.Fatal(reply)
		}

		if iole := mcm.pcm.log.GetIndexOfLastEntry(); iole != 8 {
			t.Fatal(iole)
		}
		addedLogEntry := testHelper_GetLogEntryAtIndex(mcm.pcm.log, 6)
		if addedLogEntry.TermNo != 5 {
			t.Error()
		}
		if !testCommandEquals(addedLogEntry.Command, "c6'") {
			t.Error()
		}

		if mcm.pcm.getCommitIndex() != 7 {
			t.Error()
		}

		// #RFS-F2: (paraphrasing) AppendEntries RPC from current leader should
		// prevent election timeout
		if mcm.pcm.electionTimeoutTracker.electionTimeoutTime == electionTimeoutTime1 {
			t.Fatal()
		}
	}

	// Follower
	f(testSetupMCM_Follower_WithTerms, false)
	f(testSetupMCM_Follower_WithTerms, false)

	// Candidate
	f(testSetupMCM_Candidate_WithTerms, false)
	f(testSetupMCM_Candidate_WithTerms, false)

	// Leader
	f(testSetupMCM_Leader_WithTerms, true)
	test_ExpectPanic(
		t,
		func() {
			f(testSetupMCM_Leader_WithTerms, false)
		},
		"FATAL: two leaders with same term - got AppendEntries from: s4 with term: 8",
	)
}

// Variant of TestRpcAEAppendNewEntries to test alternate path for step 5.
// Note: this test case based on Figure 7, case (b) in the Raft paper
func TestCM_RpcAE_AppendNewEntriesB(t *testing.T) {
	f := func(
		setup func(t *testing.T, terms []TermNo) (mcm *managedConsensusModule, mrs *mockRpcSender),
		senderTermIsNewer bool,
		expectedVotedFor ServerId,
	) {
		mcm, _ := setup(
			t,
			[]TermNo{1, 1, 1, 4},
		)
		mcm.pcm.setCommitIndex(3)

		serverTerm := mcm.pcm.persistentState.GetCurrentTerm()
		electionTimeoutTime1 := mcm.pcm.electionTimeoutTracker.electionTimeoutTime

		senderTerm := serverTerm
		if senderTermIsNewer {
			senderTerm += 1
		}

		if !testCommandEquals(testHelper_GetLogEntryAtIndex(mcm.pcm.log, 4).Command, "c4") {
			t.Error()
		}

		sentLogEntries := []LogEntry{
			{4, Command("c5'")},
			{5, Command("c6'")},
		}

		appendEntries := &RpcAppendEntries{senderTerm, 4, 4, sentLogEntries, 7}

		reply := mcm.rpc_RpcAppendEntries("s4", appendEntries)

		expectedRpc := RpcAppendEntriesReply{senderTerm, true}
		if *reply != expectedRpc {
			t.Fatal(reply)
		}

		if iole := mcm.pcm.log.GetIndexOfLastEntry(); iole != 6 {
			t.Fatal(iole)
		}
		addedLogEntry := testHelper_GetLogEntryAtIndex(mcm.pcm.log, 6)
		if addedLogEntry.TermNo != 5 {
			t.Error()
		}
		if !testCommandEquals(addedLogEntry.Command, "c6'") {
			t.Error()
		}

		if mcm.pcm.getCommitIndex() != 6 {
			t.Error()
		}

		if mcm.pcm.persistentState.GetVotedFor() != expectedVotedFor {
			t.Fatal()
		}

		// #RFS-F2: (paraphrasing) AppendEntries RPC from current leader should
		// prevent election timeout
		if mcm.pcm.electionTimeoutTracker.electionTimeoutTime == electionTimeoutTime1 {
			t.Fatal()
		}
	}

	// Follower
	f(testSetupMCM_Follower_WithTerms, true, "")
	f(testSetupMCM_Follower_WithTerms, false, "")

	// Candidate
	f(testSetupMCM_Candidate_WithTerms, true, "")
	f(testSetupMCM_Candidate_WithTerms, false, "s1")

	// Leader
	f(testSetupMCM_Leader_WithTerms, true, "")
	test_ExpectPanic(
		t,
		func() {
			f(testSetupMCM_Leader_WithTerms, false, "s1")
		},
		"FATAL: two leaders with same term - got AppendEntries from: s4 with term: 8",
	)

}
