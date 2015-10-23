package raft

import (
	"fmt"
	"reflect"
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

	// because Sleep(testSleepToLetGoroutineRun) can take a bit longer
	testElectionTimeoutFuzz = testSleepToLetGoroutineRun + (time.Millisecond * 3 / 2)
)

var testPeerIds = []ServerId{"s2", "s3", "s4", "s5"}

func setupTestFollower(t *testing.T, logTerms []TermNo) *ConsensusModule {
	cm, _ := setupTestFollowerR2(t, logTerms)
	return cm
}

func setupTestFollowerR2(
	t *testing.T,
	logTerms []TermNo,
) (*ConsensusModule, *mockRpcSender) {
	ps := newIMPSWithCurrentTerm(testCurrentTerm)
	imle := newIMLEWithDummyCommands(logTerms)
	mrs := newMockRpcSender()
	ts := TimeSettings{testTickerDuration, testElectionTimeoutLow}
	cm := NewConsensusModule(ps, imle, mrs, testServerId, testPeerIds, ts)
	if cm == nil {
		t.Fatal()
	}
	return cm, mrs
}

func setupManagedConsensusModule(t *testing.T, logTerms []TermNo) *managedConsensusModule {
	cm, _ := setupManagedConsensusModuleR2(t, logTerms)
	return cm
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

func (cm *ConsensusModule) stopAsyncWithRecover() (e interface{}) {
	defer func() {
		e = recover()
	}()
	cm.StopAsync()
	return nil
}

func (cm *ConsensusModule) stopAndCheckError() {
	var callersError, stopAsyncError, stopStatusError, stopError interface{}

	callersError = recover()

	stopAsyncError = cm.stopAsyncWithRecover()

	time.Sleep(testSleepToLetGoroutineRun)
	if !cm.IsStopped() {
		stopStatusError = "Timeout waiting for stop!"
	}

	stopError = cm.GetStopError()

	if stopAsyncError == nil && stopStatusError == nil && stopError == nil {
		if callersError != nil {
			panic(callersError)
		} else {
			return
		}
	} else {
		errs := [...]interface{}{
			callersError,
			stopAsyncError,
			stopStatusError,
			stopError,
		}
		panic(fmt.Sprintf("%v", errs))
	}
}

// #5.2-p1s2: When servers start up, they begin as followers
func TestCMStartsAsFollower(t *testing.T) {
	mcm := setupManagedConsensusModule(t, nil)

	if mcm.cm.GetServerState() != FOLLOWER {
		t.Fatal()
	}
}

func TestCMStop(t *testing.T) {
	cm := setupTestFollower(t, nil)

	if cm.IsStopped() {
		t.Error()
	}

	cm.StopAsync()
	time.Sleep(testSleepToLetGoroutineRun)

	if !cm.IsStopped() {
		t.Error()
	}
	if cm.GetStopError() != nil {
		t.Error()
	}
}

func TestCMUnknownRpcTypeStopsCM(t *testing.T) {
	cm := setupTestFollower(t, nil)

	if cm.IsStopped() {
		t.Error()
	}

	cm.ProcessRpcAsync("s2", &struct{ int }{42})
	time.Sleep(testSleepToLetGoroutineRun)

	if !cm.IsStopped() {
		cm.StopAsync()
		t.Fatal()
	}

	e := cm.GetStopError()
	if e != "FATAL: unknown rpc type: *struct { int }" {
		t.Error(e)
	}
}

func TestCMUnknownRpcTypePanics(t *testing.T) {
	mcm := setupManagedConsensusModule(t, nil)

	defer func() {
		if r := recover(); r != "FATAL: unknown rpc type: *struct { int }" {
			t.Error(r)
		}
	}()

	mcm.cm.rpc("s2", &struct{ int }{42})
	t.Fatal()
}

func TestCMSetServerStateBadServerStatePanics(t *testing.T) {
	mcm := setupManagedConsensusModule(t, nil)

	defer func() {
		if r := recover(); r != "FATAL: unknown ServerState: 42" {
			t.Error(r)
		}
	}()

	mcm.cm.setServerState(42)
	t.Fatal()
}

func TestCMBadServerStatePanicsTick(t *testing.T) {
	mcm := setupManagedConsensusModule(t, nil)

	mcm.cm.serverState = 42

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
func testCMFollowerStartsElectionOnElectionTimeout(
	t *testing.T,
	mcm *managedConsensusModule,
	mrs *mockRpcSender,
) {

	if mcm.cm.GetServerState() != FOLLOWER {
		t.Fatal()
	}
	if mcm.cm.persistentState.GetVotedFor() != "" {
		t.Fatal()
	}
	if mcm.cm.currentElectionTimeout < testElectionTimeoutLow || mcm.cm.currentElectionTimeout > 2*testElectionTimeoutLow {
		t.Fatal()
	}

	// Test that a tick before election timeout causes no state change.
	mcm.tick()
	if mcm.cm.persistentState.GetCurrentTerm() != testCurrentTerm {
		t.Fatal()
	}
	if mcm.cm.GetServerState() != FOLLOWER {
		t.Fatal()
	}

	testCMFollowerStartsElectionOnElectionTimeout_Part2(t, mcm, mrs, testCurrentTerm+1)
}

func testCMFollowerStartsElectionOnElectionTimeout_Part2(
	t *testing.T,
	mcm *managedConsensusModule,
	mrs *mockRpcSender,
	expectedNewTerm TermNo,
) {

	// Test that election timeout causes a new election
	mcm.tickTilElectionTimeout()
	if mcm.cm.persistentState.GetCurrentTerm() != expectedNewTerm {
		t.Fatal(expectedNewTerm, mcm.cm.persistentState.GetCurrentTerm())
	}
	if mcm.cm.GetServerState() != CANDIDATE {
		t.Fatal()
	}
	// candidate has voted for itself
	if mcm.cm.persistentState.GetVotedFor() != testServerId {
		t.Fatal()
	}

	// candidate has issued RequestVote RPCs to all other servers.

	sentRpcs := mrs.getAllSortedByToServer()

	if len(sentRpcs) != len(testPeerIds) {
		t.Error()
	}

	lastLogIndex := mcm.cm.log.getIndexOfLastEntry()
	var lastLogTerm TermNo = 0
	if lastLogIndex > 0 {
		lastLogTerm = mcm.cm.log.getTermAtIndex(lastLogIndex)
	}

	expectedRpc := &RpcRequestVote{expectedNewTerm, lastLogIndex, lastLogTerm}

	for i, peerId := range testPeerIds {
		sentRpc := sentRpcs[i]
		if sentRpc.toServer != peerId {
			t.Error()
		}
		if !reflect.DeepEqual(sentRpc.rpc, expectedRpc) {
			t.Fatal(sentRpc.rpc, expectedRpc)
		}
	}
}

func TestCMFollowerStartsElectionOnElectionTimeout_EmptyLog(t *testing.T) {
	mcm, mrs := setupManagedConsensusModuleR2(t, nil)

	testCMFollowerStartsElectionOnElectionTimeout(t, mcm, mrs)
}

func TestCMFollowerStartsElectionOnElectionTimeout_NonEmptyLog(t *testing.T) {
	// Log with 10 entries with terms as shown in Figure 7, leader line
	terms := testLogTerms_Figure7LeaderLine()
	mcm, mrs := setupManagedConsensusModuleR2(t, terms)

	testCMFollowerStartsElectionOnElectionTimeout(t, mcm, mrs)
}

// For most tests, we'll use a passive CM where we control the progress
// of time with helper methods. This simplifies tests and avoids concurrency
// issues with inspecting the internals.
type managedConsensusModule struct {
	cm  *ConsensusModule
	now time.Time
}

func (mcm *managedConsensusModule) tick() {
	mcm.cm.tick(mcm.now)
	mcm.now = mcm.now.Add(testTickerDuration)
}

func (mcm *managedConsensusModule) tickTilElectionTimeout() {
	electionTimeoutTime := mcm.cm.electionTimeoutTime
	for {
		mcm.tick()
		if mcm.now.After(electionTimeoutTime) {
			break
		}
	}
	if mcm.cm.electionTimeoutTime != electionTimeoutTime {
		panic("electionTimeoutTime changed!")
	}
	// Because tick() increments "now" after calling tick(),
	// we need one more to actually run with a post-timeout "now".
	mcm.tick()
}
