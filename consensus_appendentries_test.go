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
func TestCM_RpcAE_Follower_LeaderTermLessThanCurrentTerm(t *testing.T) {
	mcm, mrs := setupManagedConsensusModuleR2(t, nil)
	followerTerm := mcm.pcm.persistentState.GetCurrentTerm()

	appendEntries := makeAEWithTerm(followerTerm - 1)

	mcm.pcm.rpc("s2", appendEntries)

	expectedRpc := &RpcAppendEntriesReply{followerTerm, false}
	expectedRpcs := []mockSentRpc{
		{"s2", expectedRpc},
	}
	mrs.checkSentRpcs(t, expectedRpcs)
}

// 2. Reply false if log doesn't contain an entry at prevLogIndex whose term
//      matches prevLogTerm (#5.3)
// Note: the above language is slightly ambiguous but I'm assuming that this
// step refers strictly to the follower's log not having any entry at
// prevLogIndex since step 3 covers the alternate conflicting entry case.
// Note: this test based on Figure 7, server (b)
func TestCM_RpcAE_Follower_NoMatchingLogEntry(t *testing.T) {
	mcm, mrs := setupManagedConsensusModuleR2(t, []TermNo{1, 1, 1, 4})
	followerTerm := mcm.pcm.persistentState.GetCurrentTerm()

	appendEntries := makeAEWithTermAndPrevLogDetails(testCurrentTerm, 10, 6)

	mcm.pcm.rpc("s3", appendEntries)

	expectedRpc := &RpcAppendEntriesReply{followerTerm, false}
	expectedRpcs := []mockSentRpc{
		{"s3", expectedRpc},
	}
	mrs.checkSentRpcs(t, expectedRpcs)
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
func TestCM_RpcAE_Follower_AppendNewEntries(t *testing.T) {
	mcm, mrs := setupManagedConsensusModuleR2(
		t,
		[]TermNo{1, 1, 1, 4, 4, 4, 4, 4, 4, 4, 4},
	)
	mcm.pcm.volatileState.commitIndex = 3
	followerTerm := mcm.pcm.persistentState.GetCurrentTerm()

	if mcm.pcm.log.getLogEntryAtIndex(6).Command != "c6" {
		t.Error()
	}

	sentLogEntries := []LogEntry{
		{5, "c6'"},
		{5, "c7'"},
		{6, "c8'"},
	}

	appendEntries := &RpcAppendEntries{testCurrentTerm, 5, 4, sentLogEntries, 7}

	mcm.pcm.rpc("s4", appendEntries)

	expectedRpc := &RpcAppendEntriesReply{followerTerm, true}
	expectedRpcs := []mockSentRpc{
		{"s4", expectedRpc},
	}
	mrs.checkSentRpcs(t, expectedRpcs)

	if iole := mcm.pcm.log.getIndexOfLastEntry(); iole != 8 {
		t.Fatal(iole)
	}
	addedLogEntry := mcm.pcm.log.getLogEntryAtIndex(6)
	if addedLogEntry.TermNo != 5 {
		t.Error()
	}
	if addedLogEntry.Command != "c6'" {
		t.Error()
	}

	if mcm.pcm.volatileState.commitIndex != 7 {
		t.Error()
	}
}

// Variant of TestRpcAEAppendNewEntries to test alternate path for step 5.
// Note: this test case based on Figure 7, case (b) in the Raft paper
func TestCM_RpcAE_Follower_AppendNewEntriesB(t *testing.T) {
	mcm, mrs := setupManagedConsensusModuleR2(
		t,
		[]TermNo{1, 1, 1, 4},
	)
	mcm.pcm.volatileState.commitIndex = 3
	followerTerm := mcm.pcm.persistentState.GetCurrentTerm()

	if mcm.pcm.log.getLogEntryAtIndex(4).Command != "c4" {
		t.Error()
	}

	sentLogEntries := []LogEntry{
		{4, "c5'"},
		{5, "c6'"},
	}

	appendEntries := &RpcAppendEntries{testCurrentTerm, 4, 4, sentLogEntries, 7}

	mcm.pcm.rpc("s4", appendEntries)

	expectedRpc := &RpcAppendEntriesReply{followerTerm, true}
	expectedRpcs := []mockSentRpc{
		{"s4", expectedRpc},
	}
	mrs.checkSentRpcs(t, expectedRpcs)

	if iole := mcm.pcm.log.getIndexOfLastEntry(); iole != 6 {
		t.Fatal(iole)
	}
	addedLogEntry := mcm.pcm.log.getLogEntryAtIndex(6)
	if addedLogEntry.TermNo != 5 {
		t.Error()
	}
	if addedLogEntry.Command != "c6'" {
		t.Error()
	}

	if mcm.pcm.volatileState.commitIndex != 6 {
		t.Error()
	}
}
