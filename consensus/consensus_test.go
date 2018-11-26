package consensus

import (
	"log"
	"os"
	"reflect"
	"testing"
	"time"

	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/aesender"
	"github.com/divtxt/raft/config"
	"github.com/divtxt/raft/consensus/candidate"
	"github.com/divtxt/raft/internal"
	raft_log "github.com/divtxt/raft/log"
	"github.com/divtxt/raft/rps"
	"github.com/divtxt/raft/testdata"
	"github.com/divtxt/raft/testhelpers"
	"github.com/divtxt/raft/testing2"
)

func setupManagedConsensusModule(t *testing.T, logTerms []TermNo) *managedConsensusModule {
	mcm, _ := setupManagedConsensusModuleR2(t, logTerms, false)
	return mcm
}

func setupManagedConsensusModuleR2(
	t *testing.T,
	logTerms []TermNo,
	solo bool,
) (*managedConsensusModule, *testhelpers.MockRpcSender) {
	ps := rps.NewIMPSWithCurrentTerm(testdata.CurrentTerm)

	iml, err := raft_log.TestUtil_NewInMemoryLog_WithTerms(
		logTerms, testdata.MaxEntriesPerAppendEntry,
	)
	if err != nil {
		t.Fatal(err)
	}

	commitIndex := testhelpers.NewUnlockedWatchedIndex()
	mc := newMockCommitter()
	mrs := testhelpers.NewMockRpcSender()
	aes := aesender.NewLogOnlyAESender(iml, mrs.SendOnlyRpcAppendEntriesAsync)
	var allServerIds []ServerId
	if solo {
		allServerIds = []ServerId{testdata.ThisServerId}
	} else {
		allServerIds = testdata.AllServerIds
	}
	ci, err := config.NewClusterInfo(allServerIds, testdata.ThisServerId)
	if err != nil {
		t.Fatal(err)
	}
	cc := newControlledClock(time.Now())
	cm, err := NewPassiveConsensusModule(
		ps,
		iml,
		commitIndex,
		mc,
		mrs.SendOnlyRpcRequestVoteAsync,
		aes,
		ci,
		testdata.ElectionTimeoutLow,
		cc.now,
		log.New(os.Stderr, "consensus_test", log.Flags()),
	)
	if err != nil {
		t.Fatal(err)
	}
	if cm == nil {
		t.Fatal()
	}
	// Bias simulated clock to avoid exact time matches
	cc.advance(testdata.SleepToLetGoroutineRun)
	mcm := &managedConsensusModule{cm, cc, iml, mc}
	return mcm, mrs
}

// #5.2-p1s2: When servers start up, they begin as followers
func TestCM_InitialState(t *testing.T) {
	mcm := setupManagedConsensusModule(t, nil)

	if mcm.pcm.GetServerState() != FOLLOWER {
		t.Fatal()
	}

	// Volatile state on all servers
	if mcm.pcm.GetCommitIndex() != 0 {
		t.Fatal()
	}
}

func TestCM_SetServerState_BadServerStatePanics(t *testing.T) {
	mcm := setupManagedConsensusModule(t, nil)
	testing2.AssertPanicsWith(
		t,
		func() {
			mcm.pcm.setServerState(42)
		},
		"FATAL: unknown ServerState: 42",
	)
}

// #RFS-F2: If election timeout elapses without receiving
// AppendEntries RPC from current leader or granting vote
// to candidate: convert to candidate
// #5.2-p1s5: If a follower receives no communication over a period of time
// called the election timeout, then it assumes there is no viable leader
// and begins an election to choose a new leader.
// #RFS-C1: On conversion to candidate, start election:
// Increment currentTerm; Vote for self; Send RequestVote RPCs
// to all other servers; Reset election timer
// #5.2-p2s1: To begin an election, a follower increments its current term
// and transitions to candidate state.
// #5.2-p2s2: It then votes for itself and issues RequestVote RPCs in parallel
// to each of the other servers in the cluster.
// #5.2-p6s2: ..., election timeouts are chosen randomly from a fixed
// interval (e.g., 150-300ms)
func testCM_Follower_StartsElectionOnElectionTimeout(
	t *testing.T,
	mcm *managedConsensusModule,
	mrs *testhelpers.MockRpcSender,
) {

	if mcm.pcm.GetServerState() != FOLLOWER {
		t.Fatal()
	}
	if mcm.pcm.RaftPersistentState.GetVotedFor() != 0 {
		t.Fatal()
	}

	// Test that a tick before election timeout causes no state change.
	err := mcm.Tick()
	if err != nil {
		t.Fatal(err)
	}
	if mcm.pcm.RaftPersistentState.GetCurrentTerm() != testdata.CurrentTerm {
		t.Fatal()
	}
	if mcm.pcm.GetServerState() != FOLLOWER {
		t.Fatal()
	}

	testCM_FollowerOrCandidate_StartsElectionOnElectionTimeout_Part2(
		t, mcm, mrs, testdata.CurrentTerm+1,
	)
}

func testCM_FollowerOrCandidate_StartsElectionOnElectionTimeout_Part2(
	t *testing.T,
	mcm *managedConsensusModule,
	mrs *testhelpers.MockRpcSender,
	expectedNewTerm TermNo,
) {
	timeout1 := mcm.pcm.ElectionTimeoutTimer.GetCurrentDuration()
	// Test that election timeout causes a new election
	mcm.tickTilElectionTimeout(t)
	if mcm.pcm.RaftPersistentState.GetCurrentTerm() != expectedNewTerm {
		t.Fatal(expectedNewTerm, mcm.pcm.RaftPersistentState.GetCurrentTerm())
	}
	if mcm.pcm.GetServerState() != CANDIDATE {
		t.Fatal()
	}
	// candidate has voted for itself
	if mcm.pcm.RaftPersistentState.GetVotedFor() != testdata.ThisServerId {
		t.Fatal()
	}
	// a new election timeout was chosen
	// Playing the odds here :P
	if mcm.pcm.ElectionTimeoutTimer.GetCurrentDuration() == timeout1 {
		t.Fatal()
	}
	// candidate state is fresh
	expectedCvs, err := candidate.NewCandidateVolatileState(mcm.pcm.ClusterInfo)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(mcm.pcm.CandidateVolatileState, expectedCvs) {
		t.Fatal()
	}

	// candidate has issued RequestVote RPCs to all other servers.
	lastLogIndex, lastLogTerm, err := GetIndexAndTermOfLastEntry(mcm.pcm.logRO)
	if err != nil {
		t.Fatal(err)
	}
	expectedRpc := &RpcRequestVote{expectedNewTerm, lastLogIndex, lastLogTerm}
	expectedRpcs := map[ServerId]interface{}{}
	err = mcm.pcm.ClusterInfo.ForEachPeer(func(serverId ServerId) error {
		expectedRpcs[serverId] = expectedRpc
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	mrs.CheckSentRpcs(t, expectedRpcs)
	mrs.ClearSentRpcs()
}

func testCM_SOLO_Follower_ElectsSelfOnElectionTimeout(
	t *testing.T,
	mcm *managedConsensusModule,
	mrs *testhelpers.MockRpcSender,
) {
	if mcm.pcm.GetServerState() != FOLLOWER {
		t.Fatal()
	}
	if mcm.pcm.RaftPersistentState.GetVotedFor() != 0 {
		t.Fatal()
	}

	// Test that a tick before election timeout causes no state change.
	err := mcm.Tick()
	if err != nil {
		t.Fatal(err)
	}
	if mcm.pcm.RaftPersistentState.GetCurrentTerm() != testdata.CurrentTerm {
		t.Fatal()
	}
	if mcm.pcm.GetServerState() != FOLLOWER {
		t.Fatal()
	}

	var expectedNewTerm TermNo = testdata.CurrentTerm + 1

	timeout1 := mcm.pcm.ElectionTimeoutTimer.GetCurrentDuration()
	// Test that election timeout causes a new election
	mcm.tickTilElectionTimeout(t)
	if mcm.pcm.RaftPersistentState.GetCurrentTerm() != expectedNewTerm {
		t.Fatal(expectedNewTerm, mcm.pcm.RaftPersistentState.GetCurrentTerm())
	}
	// Single node should immediately elect itself as leader
	if mcm.pcm.GetServerState() != LEADER {
		t.Fatal()
	}
	// candidate has voted for itself
	if mcm.pcm.RaftPersistentState.GetVotedFor() != testdata.ThisServerId {
		t.Fatal()
	}
	// a new election timeout was chosen
	if mcm.pcm.ElectionTimeoutTimer.GetCurrentDuration() == timeout1 {
		t.Fatal()
	}
	// candidate state is fresh
	expectedCvs, err := candidate.NewCandidateVolatileState(mcm.pcm.ClusterInfo)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(mcm.pcm.CandidateVolatileState, expectedCvs) {
		t.Fatal()
	}

	// no RPCs issued
	mrs.CheckSentRpcs(t, make(map[ServerId]interface{}))
	mrs.ClearSentRpcs()
}

func TestCM_Follower_StartsElectionOnElectionTimeout_EmptyLog(t *testing.T) {
	mcm, mrs := setupManagedConsensusModuleR2(t, nil, false)
	testCM_Follower_StartsElectionOnElectionTimeout(t, mcm, mrs)
}

func TestCM_SOLO_Follower_ElectsSelfOnElectionTimeout_EmptyLog(t *testing.T) {
	mcm, mrs := setupManagedConsensusModuleR2(t, nil, true)
	testCM_SOLO_Follower_ElectsSelfOnElectionTimeout(t, mcm, mrs)
}

// #RFS-L1b: repeat during idle periods to prevent election timeout (#5.2)
func TestCM_Leader_SendEmptyAppendEntriesDuringIdlePeriods(t *testing.T) {
	mcm, mrs := testSetupMCM_Leader_Figure7LeaderLine(t)
	serverTerm := mcm.pcm.RaftPersistentState.GetCurrentTerm()
	err := mcm.pcm.setCommitIndex(6)
	if err != nil {
		t.Fatal(err)
	}

	mrs.CheckSentRpcs(t, make(map[ServerId]interface{}))
	mrs.ClearSentRpcs()

	err = mcm.Tick()
	if err != nil {
		t.Fatal(err)
	}

	testIsLeaderWithTermAndSentEmptyAppendEntries(t, mcm, mrs, serverTerm)
}

// #RFS-L3.0: If last log index >= nextIndex for a follower: send
// AppendEntries RPC with log entries starting at nextIndex
func TestCM_Leader_TickSendsAppendEntriesWithLogEntries(t *testing.T) {
	mcm, mrs := testSetupMCM_Leader_Figure7LeaderLine_WithUpToDatePeers(t)
	serverTerm := mcm.pcm.RaftPersistentState.GetCurrentTerm()
	err := mcm.pcm.setCommitIndex(5)
	if err != nil {
		t.Fatal(err)
	}

	// repatch some peers as not caught up
	fm102, err := mcm.pcm.LeaderVolatileState.GetFollowerManager(102)
	if err != nil {
		t.Fatal(err)
	}
	fm102.SetMatchIndexAndNextIndex(9)
	fm105, err := mcm.pcm.LeaderVolatileState.GetFollowerManager(105)
	if err != nil {
		t.Fatal(err)
	}
	fm105.SetMatchIndexAndNextIndex(7)

	// tick should trigger check & appropriate sends
	err = mcm.Tick()
	if err != nil {
		t.Fatal(err)
	}

	expectedRpcEmpty := &RpcAppendEntries{
		serverTerm,
		10,
		6,
		[]LogEntry{},
		5,
	}
	expectedRpcS2 := &RpcAppendEntries{
		serverTerm,
		9,
		6,
		[]LogEntry{
			{6, Command("c10")},
		},
		5,
	}
	expectedRpcS5 := &RpcAppendEntries{
		serverTerm,
		7,
		5,
		[]LogEntry{
			{6, Command("c8")},
			{6, Command("c9")},
			{6, Command("c10")},
		},
		5,
	}
	expectedRpcs := map[ServerId]interface{}{
		102: expectedRpcS2,
		103: expectedRpcEmpty,
		104: expectedRpcEmpty,
		105: expectedRpcS5,
	}
	mrs.CheckSentRpcs(t, expectedRpcs)
	mrs.ClearSentRpcs()
}

func TestCM_Leader_FM_SendAppendEntriesToPeer(t *testing.T) {
	mcm, mrs := testSetupMCM_Leader_Figure7LeaderLine(t)
	serverTerm := mcm.pcm.RaftPersistentState.GetCurrentTerm()
	err := mcm.pcm.setCommitIndex(4)
	if err != nil {
		t.Fatal(err)
	}

	// sanity check
	expectedNextIndex := map[ServerId]LogIndex{102: 11, 103: 11, 104: 11, 105: 11}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.NextIndexes(), expectedNextIndex) {
		t.Fatal()
	}

	// nothing to send
	err = mcm.testHelper_sendAppendEntriesToPeer(102, false)
	if err != nil {
		t.Fatal(err)
	}
	expectedRpc := &RpcAppendEntries{
		serverTerm,
		10,
		6,
		[]LogEntry{},
		4,
	}
	expectedRpcs := map[ServerId]interface{}{
		102: expectedRpc,
	}
	mrs.CheckSentRpcs(t, expectedRpcs)
	mrs.ClearSentRpcs()

	// empty send
	fm102, err := mcm.pcm.LeaderVolatileState.GetFollowerManager(102)
	if err != nil {
		t.Fatal(err)
	}
	err = fm102.DecrementNextIndex()
	if err != nil {
		t.Fatal(err)
	}
	err = mcm.testHelper_sendAppendEntriesToPeer(102, true)
	if err != nil {
		t.Fatal(err)
	}
	expectedRpc = &RpcAppendEntries{
		serverTerm,
		9,
		6,
		[]LogEntry{},
		4,
	}
	expectedRpcs = map[ServerId]interface{}{
		102: expectedRpc,
	}
	mrs.CheckSentRpcs(t, expectedRpcs)
	mrs.ClearSentRpcs()

	// send one
	err = mcm.testHelper_sendAppendEntriesToPeer(102, false)
	if err != nil {
		t.Fatal(err)
	}
	expectedRpc = &RpcAppendEntries{
		serverTerm,
		9,
		6,
		[]LogEntry{
			{6, Command("c10")},
		},
		4,
	}
	expectedRpcs = map[ServerId]interface{}{
		102: expectedRpc,
	}
	mrs.CheckSentRpcs(t, expectedRpcs)
	mrs.ClearSentRpcs()

	// send multiple
	err = fm102.DecrementNextIndex()
	if err != nil {
		t.Fatal(err)
	}
	err = fm102.DecrementNextIndex()
	if err != nil {
		t.Fatal(err)
	}
	err = mcm.testHelper_sendAppendEntriesToPeer(102, false)
	if err != nil {
		t.Fatal(err)
	}
	expectedRpc = &RpcAppendEntries{
		serverTerm,
		7,
		5,
		[]LogEntry{
			{6, Command("c8")},
			{6, Command("c9")},
			{6, Command("c10")},
		},
		4,
	}
	expectedRpcs = map[ServerId]interface{}{
		102: expectedRpc,
	}
	mrs.CheckSentRpcs(t, expectedRpcs)
	mrs.ClearSentRpcs()
}

// #RFS-L4: If there exists an N such that N > commitIndex, a majority
// of matchIndex[i] >= N, and log[N].term == currentTerm:
// set commitIndex = N (#5.3, #5.4)
// Note: test based on Figure 7; server is leader line; peers are other cases
func TestCM_Leader_TickAdvancesCommitIndexIfPossible(t *testing.T) {
	mcm, mrs := testSetupMCM_Leader_Figure7LeaderLine(t)
	serverTerm := mcm.pcm.RaftPersistentState.GetCurrentTerm()

	// pre checks
	if serverTerm != 8 {
		t.Fatal()
	}
	if mcm.pcm.GetCommitIndex() != 0 {
		t.Fatal()
	}
	mcm.mc.CheckCalls(nil)
	expectedRpcs := map[ServerId]interface{}{}
	mrs.CheckSentRpcs(t, expectedRpcs)
	mrs.ClearSentRpcs()

	// match peers for cases (a), (b), (c) & (d)
	fm102, err := mcm.pcm.LeaderVolatileState.GetFollowerManager(102)
	if err != nil {
		t.Fatal(err)
	}
	fm102.SetMatchIndexAndNextIndex(9)
	fm103, err := mcm.pcm.LeaderVolatileState.GetFollowerManager(103)
	if err != nil {
		t.Fatal(err)
	}
	fm103.SetMatchIndexAndNextIndex(4)
	fm104, err := mcm.pcm.LeaderVolatileState.GetFollowerManager(104)
	if err != nil {
		t.Fatal(err)
	}
	fm104.SetMatchIndexAndNextIndex(10)
	fm105, err := mcm.pcm.LeaderVolatileState.GetFollowerManager(105)
	if err != nil {
		t.Fatal(err)
	}
	fm105.SetMatchIndexAndNextIndex(10)

	// tick should try to advance commitIndex but nothing should happen
	err = mcm.Tick()
	if err != nil {
		t.Fatal(err)
	}
	if mcm.pcm.GetCommitIndex() != 0 {
		t.Fatal()
	}
	mcm.mc.CheckCalls(nil)
	expectedRpcs = map[ServerId]interface{}{
		102: &RpcAppendEntries{serverTerm, 9, 6, []LogEntry{
			{6, Command("c10")},
		}, 0},
		103: &RpcAppendEntries{serverTerm, 4, 4, []LogEntry{
			{4, Command("c5")},
			{5, Command("c6")},
			{5, Command("c7")},
		}, 0},
		104: &RpcAppendEntries{serverTerm, 10, 6, []LogEntry{}, 0},
		105: &RpcAppendEntries{serverTerm, 10, 6, []LogEntry{}, 0},
	}
	mrs.CheckSentRpcs(t, expectedRpcs)
	mrs.ClearSentRpcs()

	// let's make some new log entries
	crc11, err := mcm.pcm.AppendCommand(testhelpers.DummyCommand(11))
	if err != nil || crc11 == nil {
		t.Fatal(err)
	}
	crc12, err := mcm.pcm.AppendCommand(testhelpers.DummyCommand(12))
	if err != nil || crc12 == nil {
		t.Fatal()
	}
	mcm.mc.CheckCalls([]mockCommitterCall{
		{"RegisterListener", 11, crc11},
		{"RegisterListener", 12, crc12},
	})

	// tick should try to advance commitIndex but nothing should happen
	err = mcm.Tick()
	if err != nil {
		t.Fatal(err)
	}
	if mcm.pcm.GetCommitIndex() != 0 {
		t.Fatal()
	}
	mcm.mc.CheckCalls(nil)
	expectedRpcs = map[ServerId]interface{}{
		102: &RpcAppendEntries{serverTerm, 9, 6, []LogEntry{
			{6, Command("c10")},
			{8, Command("c11")},
			{8, Command("c12")},
		}, 0},
		103: &RpcAppendEntries{serverTerm, 4, 4, []LogEntry{
			{4, Command("c5")},
			{5, Command("c6")},
			{5, Command("c7")},
		}, 0},
		104: &RpcAppendEntries{serverTerm, 10, 6, []LogEntry{
			{8, Command("c11")},
			{8, Command("c12")},
		}, 0},
		105: &RpcAppendEntries{serverTerm, 10, 6, []LogEntry{
			{8, Command("c11")},
			{8, Command("c12")},
		}, 0},
	}
	mrs.CheckSentRpcs(t, expectedRpcs)
	mrs.ClearSentRpcs()

	// 2 peers - for cases (a) & (b) - catch up
	fm102.SetMatchIndexAndNextIndex(11)
	fm103.SetMatchIndexAndNextIndex(11)

	// tick advances commitIndex
	err = mcm.Tick()
	if err != nil {
		t.Fatal(err)
	}
	if mcm.pcm.GetCommitIndex() != 11 {
		t.Fatal()
	}
	mcm.mc.CheckCalls([]mockCommitterCall{
		{"CommitAsync", 11, nil},
	})
	expectedRpcs = map[ServerId]interface{}{
		102: &RpcAppendEntries{serverTerm, 11, 8, []LogEntry{
			{8, Command("c12")},
		}, 11},
		103: &RpcAppendEntries{serverTerm, 11, 8, []LogEntry{
			{8, Command("c12")},
		}, 11},
		104: &RpcAppendEntries{serverTerm, 10, 6, []LogEntry{
			{8, Command("c11")},
			{8, Command("c12")},
		}, 11},
		105: &RpcAppendEntries{serverTerm, 10, 6, []LogEntry{
			{8, Command("c11")},
			{8, Command("c12")},
		}, 11},
	}
	mrs.CheckSentRpcs(t, expectedRpcs)
	mrs.ClearSentRpcs()

	// replies never came back -> tick cannot advance commitIndex
	err = mcm.Tick()
	if err != nil {
		t.Fatal(err)
	}
	if mcm.pcm.GetCommitIndex() != 11 {
		t.Fatal(mcm.pcm.GetCommitIndex())
	}
	mcm.mc.CheckCalls(nil)
	mrs.CheckSentRpcs(t, expectedRpcs)
	mrs.ClearSentRpcs()
}

func TestCM_SOLO_Leader_TickAdvancesCommitIndexIfPossible(t *testing.T) {
	var err error
	mcm, mrs := testSetupMCM_SOLO_Leader_WithTerms(
		t, testdata.TestUtil_MakeFigure7LeaderLineTerms(), 0,
	)

	serverTerm := mcm.pcm.RaftPersistentState.GetCurrentTerm()

	// pre checks
	if serverTerm != 8 {
		t.Fatal()
	}
	if mcm.pcm.GetCommitIndex() != 0 {
		t.Fatal()
	}
	mcm.mc.CheckCalls(nil)
	mrs.CheckSentRpcs(t, map[ServerId]interface{}{})
	mrs.ClearSentRpcs()

	// tick should try to advance commitIndex but nothing should happen
	err = mcm.Tick()
	if err != nil {
		t.Fatal(err)
	}
	if mcm.pcm.GetCommitIndex() != 0 {
		t.Fatal()
	}
	mrs.CheckSentRpcs(t, map[ServerId]interface{}{})
	mrs.ClearSentRpcs()

	// let's make some new log entries
	crc11, err := mcm.pcm.AppendCommand(testhelpers.DummyCommand(11))
	if err != nil || crc11 == nil {
		t.Fatal()
	}
	crc12, err := mcm.pcm.AppendCommand(testhelpers.DummyCommand(12))
	if err != nil || crc12 == nil {
		t.Fatal()
	}
	mcm.mc.CheckCalls([]mockCommitterCall{
		{"RegisterListener", 11, crc11},
		{"RegisterListener", 12, crc12},
	})

	// commitIndex does not advance immediately
	if mcm.pcm.GetCommitIndex() != 0 {
		t.Fatal()
	}

	// tick will advance commitIndex to the highest match possible
	err = mcm.Tick()
	if err != nil {
		t.Fatal(err)
	}
	if mcm.pcm.GetCommitIndex() != 12 {
		t.Fatal(mcm.pcm.GetCommitIndex())
	}
	mcm.mc.CheckCalls([]mockCommitterCall{
		{"CommitAsync", 12, nil},
	})
	mrs.CheckSentRpcs(t, map[ServerId]interface{}{})
	mrs.ClearSentRpcs()
}

func TestCM_SetCommitIndexNotifiesCommitter(t *testing.T) {
	f := func(
		setup func(t *testing.T) (mcm *managedConsensusModule, mrs *testhelpers.MockRpcSender),
	) {
		mcm, _ := setup(t)

		mcm.mc.CheckCalls(nil)

		err := mcm.pcm.setCommitIndex(2)
		if err != nil {
			t.Fatal(err)
		}
		mcm.mc.CheckCalls([]mockCommitterCall{
			{"CommitAsync", 2, nil},
		})

		err = mcm.pcm.setCommitIndex(9)
		if err != nil {
			t.Fatal(err)
		}
		mcm.mc.CheckCalls([]mockCommitterCall{
			{"CommitAsync", 9, nil},
		})
	}

	f(testSetupMCM_Follower_Figure7LeaderLine)
	f(testSetupMCM_Candidate_Figure7LeaderLine)
	f(testSetupMCM_Leader_Figure7LeaderLine)
}

func testSetupMCM_Follower_WithTerms(
	t *testing.T,
	terms []TermNo,
) (*managedConsensusModule, *testhelpers.MockRpcSender) {
	mcm, mrs := setupManagedConsensusModuleR2(t, terms, false)
	if mcm.pcm.GetServerState() != FOLLOWER {
		t.Fatal()
	}
	return mcm, mrs
}

func testSetupMCM_SOLO_Follower_WithTerms(
	t *testing.T,
	terms []TermNo,
	discardEntriesBeforeIndex LogIndex,
) (*managedConsensusModule, *testhelpers.MockRpcSender) {
	mcm, mrs := setupManagedConsensusModuleR2(t, terms, true)
	if mcm.pcm.GetServerState() != FOLLOWER {
		t.Fatal()
	}
	return mcm, mrs
}

func testSetupMCM_Candidate_WithTerms(
	t *testing.T,
	terms []TermNo,
) (*managedConsensusModule, *testhelpers.MockRpcSender) {
	mcm, mrs := testSetupMCM_Follower_WithTerms(t, terms)
	testCM_Follower_StartsElectionOnElectionTimeout(t, mcm, mrs)
	if mcm.pcm.GetServerState() != CANDIDATE {
		t.Fatal()
	}
	return mcm, mrs
}

func testSetupMCM_Leader_WithTerms(
	t *testing.T,
	terms []TermNo,
) (*managedConsensusModule, *testhelpers.MockRpcSender) {
	mcm, mrs := testSetupMCM_Candidate_WithTerms(t, terms)
	serverTerm := mcm.pcm.RaftPersistentState.GetCurrentTerm()
	sentRpc := &RpcRequestVote{serverTerm, 0, 0}
	err := mcm.pcm.RpcReply_RpcRequestVoteReply(102, sentRpc, &RpcRequestVoteReply{serverTerm, true})
	if err != nil {
		t.Fatal(err)
	}
	err = mcm.pcm.RpcReply_RpcRequestVoteReply(103, sentRpc, &RpcRequestVoteReply{serverTerm, true})
	if err != nil {
		t.Fatal(err)
	}
	if mcm.pcm.GetServerState() != LEADER {
		t.Fatal()
	}
	mrs.ClearSentRpcs()
	return mcm, mrs
}

func testSetupMCM_SOLO_Leader_WithTerms(
	t *testing.T,
	terms []TermNo,
	discardEntriesBeforeIndex LogIndex,
) (*managedConsensusModule, *testhelpers.MockRpcSender) {
	mcm, mrs := testSetupMCM_SOLO_Follower_WithTerms(t, terms, discardEntriesBeforeIndex)
	testCM_SOLO_Follower_ElectsSelfOnElectionTimeout(t, mcm, mrs)
	if mcm.pcm.GetServerState() != LEADER {
		t.Fatal()
	}
	return mcm, mrs
}

func testSetupMCM_Follower_Figure7LeaderLine(
	t *testing.T,
) (*managedConsensusModule, *testhelpers.MockRpcSender) {
	return testSetupMCM_Follower_WithTerms(t, testdata.TestUtil_MakeFigure7LeaderLineTerms())
}

func testSetupMCM_Candidate_Figure7LeaderLine(
	t *testing.T,
) (*managedConsensusModule, *testhelpers.MockRpcSender) {
	return testSetupMCM_Candidate_WithTerms(t, testdata.TestUtil_MakeFigure7LeaderLineTerms())
}

func testSetupMCM_Leader_Figure7LeaderLine(
	t *testing.T,
) (*managedConsensusModule, *testhelpers.MockRpcSender) {
	return testSetupMCM_Leader_WithTerms(t, testdata.TestUtil_MakeFigure7LeaderLineTerms())
}

func testSetupMCM_Leader_Figure7LeaderLine_WithUpToDatePeers(
	t *testing.T,
) (*managedConsensusModule, *testhelpers.MockRpcSender) {
	mcm, mrs := testSetupMCM_Leader_WithTerms(t, testdata.TestUtil_MakeFigure7LeaderLineTerms())

	// sanity check - before
	expectedNextIndex := map[ServerId]LogIndex{102: 11, 103: 11, 104: 11, 105: 11}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.NextIndexes(), expectedNextIndex) {
		t.Fatal()
	}
	expectedMatchIndex := map[ServerId]LogIndex{102: 0, 103: 0, 104: 0, 105: 0}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.MatchIndexes(), expectedMatchIndex) {
		t.Fatal()
	}

	// pretend peers caught up!
	lastLogIndex, err := mcm.pcm.logRO.GetIndexOfLastEntry()
	if err != nil {
		t.Fatal()
	}
	err = mcm.pcm.ClusterInfo.ForEachPeer(
		func(serverId ServerId) error {
			fm, err := mcm.pcm.LeaderVolatileState.GetFollowerManager(serverId)
			if err != nil {
				return err
			}
			fm.SetMatchIndexAndNextIndex(lastLogIndex)
			return nil
		},
	)
	if err != nil {
		t.Fatal()
	}

	// after check
	expectedNextIndex = map[ServerId]LogIndex{102: 11, 103: 11, 104: 11, 105: 11}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.NextIndexes(), expectedNextIndex) {
		t.Fatal()
	}
	expectedMatchIndex = map[ServerId]LogIndex{102: 10, 103: 10, 104: 10, 105: 10}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.MatchIndexes(), expectedMatchIndex) {
		t.Fatal()
	}

	return mcm, mrs
}

func TestCM_Follower_StartsElectionOnElectionTimeout_NonEmptyLog(t *testing.T) {
	testSetupMCM_Candidate_Figure7LeaderLine(t)
}

// #RFS-L2: If command received from client: append entry to local log,
// respond after entry applied to state machine (#5.3)
func TestCM_Leader_AppendCommand(t *testing.T) {
	mcm, _ := testSetupMCM_Leader_Figure7LeaderLine(t)

	// pre check
	iole, err := mcm.pcm.logRO.GetIndexOfLastEntry()
	if err != nil {
		t.Fatal()
	}
	if iole != 10 {
		t.Fatal()
	}

	mcm.mc.CheckCalls(nil)
	crc1101, err := mcm.pcm.AppendCommand(testhelpers.DummyCommand(1101))
	if err != nil || crc1101 == nil {
		t.Fatal()
	}
	mcm.mc.CheckCalls([]mockCommitterCall{
		{"RegisterListener", 11, crc1101},
	})

	iole, err = mcm.pcm.logRO.GetIndexOfLastEntry()
	if err != nil {
		t.Fatal()
	}
	if iole != 11 {
		t.Fatal()
	}
	le := testhelpers.TestHelper_GetLogEntryAtIndex(mcm.log, 11)
	if !reflect.DeepEqual(le, LogEntry{8, Command("c1101")}) {
		t.Fatal(le)
	}
}

// #RFS-L2: If command received from client: append entry to local log,
// respond after entry applied to state machine (#5.3)
func TestCM_FollowerOrCandidate_AppendCommand(t *testing.T) {
	f := func(
		setup func(t *testing.T) (mcm *managedConsensusModule, mrs *testhelpers.MockRpcSender),
	) {
		mcm, _ := setup(t)

		// pre check
		iole, err := mcm.pcm.logRO.GetIndexOfLastEntry()
		if err != nil {
			t.Fatal()
		}
		if iole != 10 {
			t.Fatal()
		}
		mcm.mc.CheckCalls(nil)

		_, err = mcm.pcm.AppendCommand(testhelpers.DummyCommand(1101))
		if err != ErrNotLeader {
			t.Fatal()
		}

		iole, err = mcm.pcm.logRO.GetIndexOfLastEntry()
		if err != nil {
			t.Fatal()
		}
		if iole != 10 {
			t.Fatal()
		}
		mcm.mc.CheckCalls(nil)
	}

	f(testSetupMCM_Follower_Figure7LeaderLine)
	f(testSetupMCM_Candidate_Figure7LeaderLine)
}

// For most tests, we'll use a passive CM where we control the progress
// of time with helper methods. This simplifies tests and avoids concurrency
// issues with inspecting the internals.
type managedConsensusModule struct {
	pcm *PassiveConsensusModule
	cc  *controlledClock
	log internal.LogReadOnly
	mc  *mockCommitter
}

func (mcm *managedConsensusModule) Tick() error {
	err := mcm.pcm.Tick()
	if err != nil {
		return err
	}
	mcm.cc.advance(testdata.TickerDuration)
	return nil
}

func (mcm *managedConsensusModule) Rpc_RpcAppendEntries(
	from ServerId,
	rpc *RpcAppendEntries,
) (*RpcAppendEntriesReply, error) {
	return mcm.pcm.Rpc_RpcAppendEntries(from, rpc)
}

func (mcm *managedConsensusModule) Rpc_RpcRequestVote(
	from ServerId,
	rpc *RpcRequestVote,
) (*RpcRequestVoteReply, error) {
	return mcm.pcm.Rpc_RpcRequestVote(from, rpc)
}

func (mcm *managedConsensusModule) tickTilElectionTimeout(t *testing.T) {
	electionTimeoutTime := mcm.pcm.ElectionTimeoutTimer.GetExpiryTime()
	for {
		err := mcm.Tick()
		if err != nil {
			t.Fatal(err)
		}
		if mcm.cc._now.After(electionTimeoutTime) {
			break
		}
	}
	if mcm.pcm.ElectionTimeoutTimer.GetExpiryTime() != electionTimeoutTime {
		t.Fatal("electionTimeoutTime changed!")
	}
	// Because tick() increments "now" after calling tick(),
	// we need one more to actually run with a post-timeout "now".
	err := mcm.Tick()
	if err != nil {
		t.Fatal(err)
	}
}

func (mcm *managedConsensusModule) makeAEWithTerm(peer ServerId) *RpcAppendEntries {
	serverTerm := mcm.pcm.RaftPersistentState.GetCurrentTerm()
	fm, err := mcm.pcm.LeaderVolatileState.GetFollowerManager(peer)
	if err != nil {
		panic(err)
	}
	peerNextIndex := fm.GetNextIndex()
	return &RpcAppendEntries{
		serverTerm,
		peerNextIndex - 1,
		0,
		nil,
		0,
	}
}

// FIXME: inline as a closure since it is only used in one place (or check for other users)
func (mcm *managedConsensusModule) testHelper_sendAppendEntriesToPeer(
	peerId ServerId, empty bool,
) error {
	currentTerm := mcm.pcm.RaftPersistentState.GetCurrentTerm()
	commitIndex := mcm.pcm.GetCommitIndex()
	fm, err := mcm.pcm.LeaderVolatileState.GetFollowerManager(peerId)
	if err != nil {
		return err
	}
	return fm.SendAppendEntriesToPeerAsync(
		empty,
		currentTerm,
		commitIndex,
	)
}

// --

type controlledClock struct {
	_now time.Time
}

func newControlledClock(now time.Time) *controlledClock {
	return &controlledClock{now}
}

func (cc *controlledClock) now() time.Time {
	return cc._now
}

func (cc *controlledClock) advance(d time.Duration) {
	cc._now = cc._now.Add(d)
}
