package raft

import (
	"reflect"
	"testing"
	"time"
)

func makeAEWithTerm(term TermNo) *AppendEntries {
	return &AppendEntries{term, 0, 0, nil, 0}
}

func makeAEWithTermAndPrevLogDetails(term TermNo, prevli LogIndex, prevterm TermNo) *AppendEntries {
	return &AppendEntries{term, prevli, prevterm, nil, 0}
}

// 1. Reply false if term < currentTerm (#5.1)
func TestRpcAELeaderTermLessThanCurrentTerm(t *testing.T) {
	follower, mrs := setupTestFollowerR2(t, nil)
	defer follower.StopAsync()
	followerTerm := follower.persistentState.GetCurrentTerm()

	appendEntries := makeAEWithTerm(followerTerm - 1)

	time.Sleep(2 * testElectionTimeoutFuzz)
	follower.ProcessRpcAsync("s2", appendEntries)
	time.Sleep(testSleepToLetGoroutineRun)

	sentRpcs := mrs.getAllSortedByToServer()
	if len(sentRpcs) != 1 {
		t.Error(len(sentRpcs))
	}
	sentRpc := sentRpcs[0]
	if sentRpc.toServer != "s2" {
		t.Error()
	}
	expectedRpc := &AppendEntriesReply{followerTerm, false}
	if !reflect.DeepEqual(sentRpc.rpc, expectedRpc) {
		t.Fatal(sentRpc.rpc, expectedRpc)
	}
}

// 2. Reply false if log doesn't contain an entry at prevLogIndex whose term
//      matches prevLogTerm (#5.3)
// Note: the above language is slightly ambiguous but I'm assuming that this
// step refers strictly to the follower's log not having any entry at
// prevLogIndex since step 3 covers the alternate conflicting entry case.
// Note: this test based on Figure 7, server (b)
func TestRpcAENoMatchingLogEntry(t *testing.T) {
	follower, mrs := setupTestFollowerR2(t, []TermNo{1, 1, 1, 4})
	defer follower.StopAsync()
	followerTerm := follower.persistentState.GetCurrentTerm()

	appendEntries := makeAEWithTermAndPrevLogDetails(testCurrentTerm, 10, 6)

	follower.ProcessRpcAsync("s3", appendEntries)
	time.Sleep(testSleepToLetGoroutineRun)
	sentRpcs := mrs.getAllSortedByToServer()
	if len(sentRpcs) != 1 {
		t.Error()
	}
	sentRpc := sentRpcs[0]
	if sentRpc.toServer != "s3" {
		t.Error()
	}
	expectedRpc := &AppendEntriesReply{followerTerm, false}
	if !reflect.DeepEqual(sentRpc.rpc, expectedRpc) {
		t.Fatal(sentRpc.rpc, expectedRpc)
	}
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
func TestRpcAEAppendNewEntries(t *testing.T) {
	follower, mrs := setupTestFollowerR2(
		t,
		[]TermNo{1, 1, 1, 4, 4, 4, 4, 4, 4, 4, 4},
	)
	defer follower.StopAsync()
	follower.volatileState.commitIndex = 3
	followerTerm := follower.persistentState.GetCurrentTerm()

	if follower.log.getLogEntryAtIndex(6).Command != "c6" {
		t.Error()
	}

	sentLogEntries := []LogEntry{
		LogEntry{5, "c6'"},
		LogEntry{5, "c7'"},
		LogEntry{6, "c8'"},
	}

	appendEntries := &AppendEntries{testCurrentTerm, 5, 4, sentLogEntries, 7}

	follower.ProcessRpcAsync("s4", appendEntries)
	time.Sleep(testSleepToLetGoroutineRun)
	sentRpcs := mrs.getAllSortedByToServer()
	if len(sentRpcs) != 1 {
		t.Error()
	}
	sentRpc := sentRpcs[0]
	if sentRpc.toServer != "s4" {
		t.Error()
	}
	expectedRpc := &AppendEntriesReply{followerTerm, true}
	if !reflect.DeepEqual(sentRpc.rpc, expectedRpc) {
		t.Fatal(sentRpc.rpc, expectedRpc)
	}

	if iole := follower.log.getIndexOfLastEntry(); iole != 8 {
		t.Fatal(iole)
	}
	addedLogEntry := follower.log.getLogEntryAtIndex(6)
	if addedLogEntry.TermNo != 5 {
		t.Error()
	}
	if addedLogEntry.Command != "c6'" {
		t.Error()
	}

	// FIXME: need to test permutations in step 5
	if follower.volatileState.commitIndex != 7 {
		t.Error()
	}
}
