package raft

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

const (
	testThisServerId = "s1"
	// Note: value for tests based on Figure 7
	testCurrentTerm = 8

	testTickerDuration     = 15 * time.Millisecond
	testElectionTimeoutLow = 150 * time.Millisecond

	testSleepToLetGoroutineRun = 3 * time.Millisecond
	testSleepJustMoreThanATick = testTickerDuration + testSleepToLetGoroutineRun
)

var testAllServerIds = []ServerId{testThisServerId, "s2", "s3", "s4", "s5"}

func setupManagedConsensusModule(t *testing.T, logTerms []TermNo) *managedConsensusModule {
	mcm, _ := setupManagedConsensusModuleR2(t, logTerms)
	return mcm
}

func setupManagedConsensusModuleR2(
	t *testing.T,
	logTerms []TermNo,
) (*managedConsensusModule, *mockRpcSender) {
	ps := newIMPSWithCurrentTerm(testCurrentTerm)
	imle := newIMLEWithDummyCommands(logTerms)
	mrs := newMockRpcSender()
	ts := TimeSettings{testTickerDuration, testElectionTimeoutLow}
	ci := NewClusterInfo(testAllServerIds, testThisServerId)
	cm, now := newPassiveConsensusModule(ps, imle, mrs, ci, ts)
	if cm == nil {
		t.Fatal()
	}
	// Bias simulated clock to avoid exact time matches
	now = now.Add(testSleepToLetGoroutineRun)
	mcm := &managedConsensusModule{cm, now}
	return mcm, mrs
}

// #5.2-p1s2: When servers start up, they begin as followers
func TestCM_InitialState(t *testing.T) {
	mcm := setupManagedConsensusModule(t, nil)

	if mcm.pcm.getServerState() != FOLLOWER {
		t.Fatal()
	}

	// Volatile state on all servers
	if mcm.pcm.volatileState != (volatileState{0, 0}) {
		t.Fatal()
	}
}

func test_ExpectPanic(t *testing.T, f func(), expectedRecover interface{}) {
	skipRecover := false
	defer func() {
		if !skipRecover {
			if r := recover(); r != expectedRecover {
				t.Fatal(fmt.Sprintf("Expected panic: %v; got: %v", expectedRecover, r))
			}
		}
	}()

	f()
	skipRecover = true
	t.Fatal(fmt.Sprintf("Expected panic: %v", expectedRecover))
}

func testCM_setupMCMAndExpectPanicFor(
	t *testing.T,
	f func(*managedConsensusModule),
	expectedRecover interface{},
) {
	mcm := setupManagedConsensusModule(t, nil)
	test_ExpectPanic(
		t,
		func() {
			f(mcm)
		},
		expectedRecover,
	)
}

func TestCM_UnknownRpcTypePanics(t *testing.T) {
	testCM_setupMCMAndExpectPanicFor(
		t,
		func(mcm *managedConsensusModule) {
			mcm.rpc("s2", &struct{ int }{42})
		},
		"FATAL: unknown rpc type: *struct { int } from: s2",
	)
}

func TestCM_RpcReply_UnknownRpcTypePanics(t *testing.T) {
	testCM_setupMCMAndExpectPanicFor(
		t,
		func(mcm *managedConsensusModule) {
			mcm.pcm.rpcReply("s2", &struct{ int }{42}, &struct{ int }{42})
		},
		"FATAL: unknown rpc type: *struct { int } from: s2",
	)
}

func TestCM_RpcReply_UnknownRpcReplyTypePanics(t *testing.T) {
	testCM_setupMCMAndExpectPanicFor(
		t,
		func(mcm *managedConsensusModule) {
			sentRpc := &RpcRequestVote{1, 0, 0}
			mcm.pcm.rpcReply("s2", sentRpc, &RpcAppendEntriesReply{1, false})
		},
		"FATAL: mismatched rpcReply type: *raft.RpcAppendEntriesReply from: s2 - expected *raft.RpcRequestVoteReply",
	)
	testCM_setupMCMAndExpectPanicFor(
		t,
		func(mcm *managedConsensusModule) {
			sentRpc := &RpcAppendEntries{1, 0, 0, nil, 0}
			mcm.pcm.rpcReply("s2", sentRpc, &RpcRequestVoteReply{1, false})
		},
		"FATAL: mismatched rpcReply type: *raft.RpcRequestVoteReply from: s2 - expected *raft.RpcAppendEntriesReply",
	)
}

func TestCM_SetServerStateBadServerStatePanics(t *testing.T) {
	testCM_setupMCMAndExpectPanicFor(
		t,
		func(mcm *managedConsensusModule) {
			mcm.pcm._setServerState(42)
		},
		"FATAL: unknown ServerState: 42",
	)
}

func TestCM_BadServerStatePanicsTick(t *testing.T) {
	testCM_setupMCMAndExpectPanicFor(
		t,
		func(mcm *managedConsensusModule) {
			mcm.pcm._unsafe_serverState = 42

			mcm.tick()
		},
		"FATAL: unknown ServerState: 42",
	)
}

func TestCM_BadServerStatePanicsRpc(t *testing.T) {
	testCM_setupMCMAndExpectPanicFor(
		t,
		func(mcm *managedConsensusModule) {
			mcm.pcm._unsafe_serverState = 42

			requestVote := &RpcRequestVote{1, 0, 0}

			mcm.rpc("s2", requestVote)
		},
		"FATAL: unknown ServerState: 42",
	)
}

// #5.2-p1s5: If a follower receives no communication over a period of time
// called the election timeout, then it assumes there is no viable leader
// and begins an election to choose a new leader.
// #5.2-p2s1: To begin an election, a follower increments its current term
// and transitions to candidate state.
// #5.2-p2s2: It then votes for itself and issues RequestVote RPCs in parallel
// to each of the other servers in the cluster.
// #5.2-p6s2: ..., election timeouts are chosen randomly from a fixed
// interval (e.g., 150-300ms)
func testCM_Follower_StartsElectionOnElectionTimeout(
	t *testing.T,
	mcm *managedConsensusModule,
	mrs *mockRpcSender,
) {

	if mcm.pcm.getServerState() != FOLLOWER {
		t.Fatal()
	}
	if mcm.pcm.persistentState.GetVotedFor() != "" {
		t.Fatal()
	}

	// Test that a tick before election timeout causes no state change.
	mcm.tick()
	if mcm.pcm.persistentState.GetCurrentTerm() != testCurrentTerm {
		t.Fatal()
	}
	if mcm.pcm.getServerState() != FOLLOWER {
		t.Fatal()
	}

	testCM_FollowerOrCandidate_StartsElectionOnElectionTimeout_Part2(t, mcm, mrs, testCurrentTerm+1)
}

func testCM_FollowerOrCandidate_StartsElectionOnElectionTimeout_Part2(
	t *testing.T,
	mcm *managedConsensusModule,
	mrs *mockRpcSender,
	expectedNewTerm TermNo,
) {
	timeout1 := mcm.pcm.electionTimeoutTracker.currentElectionTimeout
	// Test that election timeout causes a new election
	mcm.tickTilElectionTimeout()
	if mcm.pcm.persistentState.GetCurrentTerm() != expectedNewTerm {
		t.Fatal(expectedNewTerm, mcm.pcm.persistentState.GetCurrentTerm())
	}
	if mcm.pcm.getServerState() != CANDIDATE {
		t.Fatal()
	}
	// candidate has voted for itself
	if mcm.pcm.persistentState.GetVotedFor() != testThisServerId {
		t.Fatal()
	}
	// a new election timeout was chosen
	// Playing the odds here :P
	if mcm.pcm.electionTimeoutTracker.currentElectionTimeout == timeout1 {
		t.Fatal()
	}
	// candidate state is fresh
	expectedCvs := newCandidateVolatileState(mcm.pcm.clusterInfo)
	if !reflect.DeepEqual(mcm.pcm.candidateVolatileState, expectedCvs) {
		t.Fatal()
	}

	// candidate has issued RequestVote RPCs to all other servers.
	lastLogIndex, lastLogTerm := getIndexAndTermOfLastEntry(mcm.pcm.log)
	expectedRpc := &RpcRequestVote{expectedNewTerm, lastLogIndex, lastLogTerm}
	expectedRpcs := []mockSentRpc{
		{"s2", expectedRpc},
		{"s3", expectedRpc},
		{"s4", expectedRpc},
		{"s5", expectedRpc},
	}
	mrs.checkSentRpcs(t, expectedRpcs)
}

func TestCM_Follower_StartsElectionOnElectionTimeout_EmptyLog(t *testing.T) {
	mcm, mrs := setupManagedConsensusModuleR2(t, nil)
	testCM_Follower_StartsElectionOnElectionTimeout(t, mcm, mrs)
}

// #RFS-L1b: repeat during idle periods to prevent election timeout (#5.2)
func TestCM_Leader_SendEmptyAppendEntriesDuringIdlePeriods(t *testing.T) {
	mcm, mrs := testSetupMCM_Leader_Figure7LeaderLine(t)
	serverTerm := mcm.pcm.persistentState.GetCurrentTerm()

	mrs.checkSentRpcs(t, []mockSentRpc{})

	mcm.tick()

	testIsLeaderWithTermAndSentEmptyAppendEntries(t, mcm, mrs, serverTerm)
}

func testSetupMCM_Follower_WithTerms(
	t *testing.T,
	terms []TermNo,
) (*managedConsensusModule, *mockRpcSender) {
	mcm, mrs := setupManagedConsensusModuleR2(t, terms)
	if mcm.pcm.getServerState() != FOLLOWER {
		t.Fatal()
	}
	return mcm, mrs
}

func testSetupMCM_Candidate_WithTerms(
	t *testing.T,
	terms []TermNo,
) (*managedConsensusModule, *mockRpcSender) {
	mcm, mrs := testSetupMCM_Follower_WithTerms(t, terms)
	testCM_Follower_StartsElectionOnElectionTimeout(t, mcm, mrs)
	if mcm.pcm.getServerState() != CANDIDATE {
		t.Fatal()
	}
	return mcm, mrs
}

func testSetupMCM_Leader_WithTerms(
	t *testing.T,
	terms []TermNo,
) (*managedConsensusModule, *mockRpcSender) {
	mcm, mrs := testSetupMCM_Candidate_WithTerms(t, terms)
	serverTerm := mcm.pcm.persistentState.GetCurrentTerm()
	sentRpc := &RpcRequestVote{serverTerm, 0, 0}
	mcm.pcm.rpcReply("s2", sentRpc, &RpcRequestVoteReply{serverTerm, true})
	mcm.pcm.rpcReply("s3", sentRpc, &RpcRequestVoteReply{serverTerm, true})
	if mcm.pcm.getServerState() != LEADER {
		t.Fatal()
	}
	mrs.clearSentRpcs()
	return mcm, mrs
}

func testSetupMCM_Follower_Figure7LeaderLine(t *testing.T) (*managedConsensusModule, *mockRpcSender) {
	return testSetupMCM_Follower_WithTerms(t, makeLogTerms_Figure7LeaderLine())
}

func testSetupMCM_Candidate_Figure7LeaderLine(t *testing.T) (*managedConsensusModule, *mockRpcSender) {
	return testSetupMCM_Candidate_WithTerms(t, makeLogTerms_Figure7LeaderLine())
}

func testSetupMCM_Leader_Figure7LeaderLine(t *testing.T) (*managedConsensusModule, *mockRpcSender) {
	return testSetupMCM_Leader_WithTerms(t, makeLogTerms_Figure7LeaderLine())
}

func TestCM_Follower_StartsElectionOnElectionTimeout_NonEmptyLog(t *testing.T) {
	testSetupMCM_Candidate_Figure7LeaderLine(t)
}

// For most tests, we'll use a passive CM where we control the progress
// of time with helper methods. This simplifies tests and avoids concurrency
// issues with inspecting the internals.
type managedConsensusModule struct {
	pcm *passiveConsensusModule
	now time.Time
}

func (mcm *managedConsensusModule) tick() {
	mcm.pcm.tick(mcm.now)
	mcm.now = mcm.now.Add(testTickerDuration)
}

func (mcm *managedConsensusModule) rpc(
	from ServerId,
	rpc interface{},
) interface{} {
	return mcm.pcm.rpc(from, rpc, mcm.now)
}

func (mcm *managedConsensusModule) tickTilElectionTimeout() {
	electionTimeoutTime := mcm.pcm.electionTimeoutTracker.electionTimeoutTime
	for {
		mcm.tick()
		if mcm.now.After(electionTimeoutTime) {
			break
		}
	}
	if mcm.pcm.electionTimeoutTracker.electionTimeoutTime != electionTimeoutTime {
		panic("electionTimeoutTime changed!")
	}
	// Because tick() increments "now" after calling tick(),
	// we need one more to actually run with a post-timeout "now".
	mcm.tick()
}
