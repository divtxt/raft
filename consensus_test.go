package raft

import (
	"testing"
	"time"
)

const (
	testServerId = "s1"
	// Note: value for tests based on Figure 7
	testCurrentTerm = 8

	testTickerDuration     = 15 * time.Millisecond
	testElectionTimeoutLow = 150 * time.Millisecond

	testSleepToLetGoroutineRun = 3 * time.Millisecond
	testSleepJustMoreThanATick = testTickerDuration + testSleepToLetGoroutineRun
)

var testPeerIds = []ServerId{"s2", "s3", "s4", "s5"}

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
	cm, now := newPassiveConsensusModule(ps, imle, mrs, testServerId, testPeerIds, ts)
	if cm == nil {
		t.Fatal()
	}
	// Bias simulated clock to avoid exact time matches
	now = now.Add(testSleepToLetGoroutineRun)
	mcm := &managedConsensusModule{cm, now}
	return mcm, mrs
}

// #5.2-p1s2: When servers start up, they begin as followers
func TestCM_StartsAsFollower(t *testing.T) {
	mcm := setupManagedConsensusModule(t, nil)

	if mcm.pcm.getServerState() != FOLLOWER {
		t.Fatal()
	}
}

func TestCM_UnknownRpcTypePanics(t *testing.T) {
	mcm := setupManagedConsensusModule(t, nil)

	defer func() {
		if r := recover(); r != "FATAL: unknown rpc type: *struct { int }" {
			t.Error(r)
		}
	}()

	mcm.pcm.rpc("s2", &struct{ int }{42})
	t.Fatal()
}

func TestCM_SetServerStateBadServerStatePanics(t *testing.T) {
	mcm := setupManagedConsensusModule(t, nil)

	defer func() {
		if r := recover(); r != "FATAL: unknown ServerState: 42" {
			t.Error(r)
		}
	}()

	mcm.pcm.setServerState(42)
	t.Fatal()
}

func TestCM_BadServerStatePanicsTick(t *testing.T) {
	mcm := setupManagedConsensusModule(t, nil)

	mcm.pcm.serverState = 42

	defer func() {
		if r := recover(); r != "FATAL: unknown ServerState: 42" {
			t.Error(r)
		}
	}()

	mcm.tick()
	t.Fatal()
}

// TODO
// func TestCMBadServerStatePanicsRpc(t *testing.T) {
// }

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
	if mcm.pcm.currentElectionTimeout < testElectionTimeoutLow || mcm.pcm.currentElectionTimeout > 2*testElectionTimeoutLow {
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
	// Test that election timeout causes a new election
	mcm.tickTilElectionTimeout()
	if mcm.pcm.persistentState.GetCurrentTerm() != expectedNewTerm {
		t.Fatal(expectedNewTerm, mcm.pcm.persistentState.GetCurrentTerm())
	}
	if mcm.pcm.getServerState() != CANDIDATE {
		t.Fatal()
	}
	// candidate has voted for itself
	if mcm.pcm.persistentState.GetVotedFor() != testServerId {
		t.Fatal()
	}

	// candidate has issued RequestVote RPCs to all other servers.
	lastLogIndex := mcm.pcm.log.getIndexOfLastEntry()
	var lastLogTerm TermNo = 0
	if lastLogIndex > 0 {
		lastLogTerm = mcm.pcm.log.getTermAtIndex(lastLogIndex)
	}
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

func testSetupCM_Candidate_Figure7LeaderLine(t *testing.T) (*managedConsensusModule, *mockRpcSender) {
	terms := makeLogTerms_Figure7LeaderLine()
	mcm, mrs := setupManagedConsensusModuleR2(t, terms)
	testCM_Follower_StartsElectionOnElectionTimeout(t, mcm, mrs)
	return mcm, mrs
}

func TestCM_Follower_StartsElectionOnElectionTimeout_NonEmptyLog(t *testing.T) {
	testSetupCM_Candidate_Figure7LeaderLine(t)
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

func (mcm *managedConsensusModule) tickTilElectionTimeout() {
	electionTimeoutTime := mcm.pcm.electionTimeoutTime
	for {
		mcm.tick()
		if mcm.now.After(electionTimeoutTime) {
			break
		}
	}
	if mcm.pcm.electionTimeoutTime != electionTimeoutTime {
		panic("electionTimeoutTime changed!")
	}
	// Because tick() increments "now" after calling tick(),
	// we need one more to actually run with a post-timeout "now".
	mcm.tick()
}
