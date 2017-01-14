package consensus

import (
	"reflect"
	"testing"
	"time"

	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/config"
	consensus_state "github.com/divtxt/raft/consensus/state"
	"github.com/divtxt/raft/log"
	"github.com/divtxt/raft/rps"
	"github.com/divtxt/raft/testdata"
	"github.com/divtxt/raft/testhelpers"
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
	iml := log.TestUtil_NewInMemoryLog_WithTerms(logTerms)
	dsm := testhelpers.NewDummyStateMachine()
	mrs := testhelpers.NewMockRpcSender()
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
	now := time.Now()
	cm, err := NewPassiveConsensusModule(
		ps,
		iml,
		dsm,
		mrs,
		ci,
		testdata.MaxEntriesPerAppendEntry,
		testdata.ElectionTimeoutLow,
		now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if cm == nil {
		t.Fatal()
	}
	// Bias simulated clock to avoid exact time matches
	now = now.Add(testdata.SleepToLetGoroutineRun)
	mcm := &managedConsensusModule{cm, now, iml, dsm}
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
	testhelpers.TestHelper_ExpectPanic(
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
	if mcm.pcm.RaftPersistentState.GetVotedFor() != "" {
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

	testCM_FollowerOrCandidate_StartsElectionOnElectionTimeout_Part2(t, mcm, mrs, testdata.CurrentTerm+1)
}

func testCM_FollowerOrCandidate_StartsElectionOnElectionTimeout_Part2(
	t *testing.T,
	mcm *managedConsensusModule,
	mrs *testhelpers.MockRpcSender,
	expectedNewTerm TermNo,
) {
	timeout1 := mcm.pcm.ElectionTimeoutTracker.GetCurrentElectionTimeout()
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
	if mcm.pcm.ElectionTimeoutTracker.GetCurrentElectionTimeout() == timeout1 {
		t.Fatal()
	}
	// candidate state is fresh
	expectedCvs, err := consensus_state.NewCandidateVolatileState(mcm.pcm.ClusterInfo)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(mcm.pcm.CandidateVolatileState, expectedCvs) {
		t.Fatal()
	}

	// candidate has issued RequestVote RPCs to all other servers.
	lastLogIndex, lastLogTerm, err := GetIndexAndTermOfLastEntry(mcm.pcm.LogRO)
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
	if mcm.pcm.RaftPersistentState.GetVotedFor() != "" {
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

	timeout1 := mcm.pcm.ElectionTimeoutTracker.GetCurrentElectionTimeout()
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
	if mcm.pcm.ElectionTimeoutTracker.GetCurrentElectionTimeout() == timeout1 {
		t.Fatal()
	}
	// candidate state is fresh
	expectedCvs, err := consensus_state.NewCandidateVolatileState(mcm.pcm.ClusterInfo)
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
	err = mcm.pcm.LeaderVolatileState.SetMatchIndexAndNextIndex("s2", 9)
	if err != nil {
		t.Fatal(err)
	}
	err = mcm.pcm.LeaderVolatileState.SetMatchIndexAndNextIndex("s5", 7)
	if err != nil {
		t.Fatal(err)
	}

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
		"s2": expectedRpcS2,
		"s3": expectedRpcEmpty,
		"s4": expectedRpcEmpty,
		"s5": expectedRpcS5,
	}
	mrs.CheckSentRpcs(t, expectedRpcs)
	mrs.ClearSentRpcs()
}

func TestCM_sendAppendEntriesToPeer(t *testing.T) {
	mcm, mrs := testSetupMCM_Leader_Figure7LeaderLine(t)
	serverTerm := mcm.pcm.RaftPersistentState.GetCurrentTerm()
	err := mcm.pcm.setCommitIndex(4)
	if err != nil {
		t.Fatal(err)
	}

	// sanity check
	expectedNextIndex := map[ServerId]LogIndex{"s2": 11, "s3": 11, "s4": 11, "s5": 11}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.NextIndex, expectedNextIndex) {
		t.Fatal()
	}

	// nothing to send
	err = mcm.pcm.sendAppendEntriesToPeer("s2", false)
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
		"s2": expectedRpc,
	}
	mrs.CheckSentRpcs(t, expectedRpcs)
	mrs.ClearSentRpcs()

	// empty send
	err = mcm.pcm.LeaderVolatileState.DecrementNextIndex("s2")
	if err != nil {
		t.Fatal(err)
	}
	err = mcm.pcm.sendAppendEntriesToPeer("s2", true)
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
		"s2": expectedRpc,
	}
	mrs.CheckSentRpcs(t, expectedRpcs)
	mrs.ClearSentRpcs()

	// send one
	err = mcm.pcm.sendAppendEntriesToPeer("s2", false)
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
		"s2": expectedRpc,
	}
	mrs.CheckSentRpcs(t, expectedRpcs)
	mrs.ClearSentRpcs()

	// send multiple
	err = mcm.pcm.LeaderVolatileState.DecrementNextIndex("s2")
	if err != nil {
		t.Fatal(err)
	}
	err = mcm.pcm.LeaderVolatileState.DecrementNextIndex("s2")
	if err != nil {
		t.Fatal(err)
	}
	err = mcm.pcm.sendAppendEntriesToPeer("s2", false)
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
		"s2": expectedRpc,
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
	expectedRpcs := map[ServerId]interface{}{}
	mrs.CheckSentRpcs(t, expectedRpcs)
	mrs.ClearSentRpcs()

	// match peers for cases (a), (b), (c) & (d)
	err := mcm.pcm.LeaderVolatileState.SetMatchIndexAndNextIndex("s2", 9)
	if err != nil {
		t.Fatal(err)
	}
	err = mcm.pcm.LeaderVolatileState.SetMatchIndexAndNextIndex("s3", 4)
	if err != nil {
		t.Fatal(err)
	}
	err = mcm.pcm.LeaderVolatileState.SetMatchIndexAndNextIndex("s4", 10)
	if err != nil {
		t.Fatal(err)
	}
	err = mcm.pcm.LeaderVolatileState.SetMatchIndexAndNextIndex("s5", 10)
	if err != nil {
		t.Fatal(err)
	}

	// tick should try to advance commitIndex but nothing should happen
	err = mcm.Tick()
	if err != nil {
		t.Fatal(err)
	}
	if mcm.pcm.GetCommitIndex() != 0 {
		t.Fatal()
	}
	expectedRpcs = map[ServerId]interface{}{
		"s2": &RpcAppendEntries{serverTerm, 9, 6, []LogEntry{
			{6, Command("c10")},
		}, 0},
		"s3": &RpcAppendEntries{serverTerm, 4, 4, []LogEntry{
			{4, Command("c5")},
			{5, Command("c6")},
			{5, Command("c7")},
		}, 0},
		"s4": &RpcAppendEntries{serverTerm, 10, 6, []LogEntry{}, 0},
		"s5": &RpcAppendEntries{serverTerm, 10, 6, []LogEntry{}, 0},
	}
	mrs.CheckSentRpcs(t, expectedRpcs)
	mrs.ClearSentRpcs()

	// let's make some new log entries
	ioleAC, err := mcm.pcm.AppendCommand(testhelpers.DummyCommand(11))
	if err != nil || ioleAC != 11 {
		t.Fatal(err)
	}
	ioleAC, err = mcm.pcm.AppendCommand(testhelpers.DummyCommand(12))
	if err != nil || ioleAC != 12 {
		t.Fatal()
	}

	// tick should try to advance commitIndex but nothing should happen
	err = mcm.Tick()
	if err != nil {
		t.Fatal(err)
	}
	if mcm.pcm.GetCommitIndex() != 0 {
		t.Fatal()
	}
	expectedRpcs = map[ServerId]interface{}{
		"s2": &RpcAppendEntries{serverTerm, 9, 6, []LogEntry{
			{6, Command("c10")},
			{8, Command("c11")},
			{8, Command("c12")},
		}, 0},
		"s3": &RpcAppendEntries{serverTerm, 4, 4, []LogEntry{
			{4, Command("c5")},
			{5, Command("c6")},
			{5, Command("c7")},
		}, 0},
		"s4": &RpcAppendEntries{serverTerm, 10, 6, []LogEntry{
			{8, Command("c11")},
			{8, Command("c12")},
		}, 0},
		"s5": &RpcAppendEntries{serverTerm, 10, 6, []LogEntry{
			{8, Command("c11")},
			{8, Command("c12")},
		}, 0},
	}
	mrs.CheckSentRpcs(t, expectedRpcs)
	mrs.ClearSentRpcs()

	// 2 peers - for cases (a) & (b) - catch up
	err = mcm.pcm.LeaderVolatileState.SetMatchIndexAndNextIndex("s2", 11)
	if err != nil {
		t.Fatal(err)
	}
	err = mcm.pcm.LeaderVolatileState.SetMatchIndexAndNextIndex("s3", 11)
	if err != nil {
		t.Fatal(err)
	}

	// tick advances commitIndex
	err = mcm.Tick()
	if err != nil {
		t.Fatal(err)
	}
	if mcm.pcm.GetCommitIndex() != 11 {
		t.Fatal()
	}
	expectedRpcs = map[ServerId]interface{}{
		"s2": &RpcAppendEntries{serverTerm, 11, 8, []LogEntry{
			{8, Command("c12")},
		}, 11},
		"s3": &RpcAppendEntries{serverTerm, 11, 8, []LogEntry{
			{8, Command("c12")},
		}, 11},
		"s4": &RpcAppendEntries{serverTerm, 10, 6, []LogEntry{
			{8, Command("c11")},
			{8, Command("c12")},
		}, 11},
		"s5": &RpcAppendEntries{serverTerm, 10, 6, []LogEntry{
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
	mrs.CheckSentRpcs(t, expectedRpcs)
	mrs.ClearSentRpcs()
}

func TestCM_SOLO_Leader_TickAdvancesCommitIndexIfPossible(t *testing.T) {
	var err error
	mcm, mrs := testSetupMCM_SOLO_Leader_WithTerms(t, testdata.TestUtil_MakeFigure7LeaderLineTerms())

	serverTerm := mcm.pcm.RaftPersistentState.GetCurrentTerm()

	// pre checks
	if serverTerm != 8 {
		t.Fatal()
	}
	if mcm.pcm.GetCommitIndex() != 0 {
		t.Fatal()
	}
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
	ioleAC, err := mcm.pcm.AppendCommand(testhelpers.DummyCommand(11))
	if err != nil || ioleAC != 11 {
		t.Fatal()
	}
	ioleAC, err = mcm.pcm.AppendCommand(testhelpers.DummyCommand(12))
	if err != nil || ioleAC != 12 {
		t.Fatal()
	}

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
	mrs.CheckSentRpcs(t, map[ServerId]interface{}{})
	mrs.ClearSentRpcs()
}

func TestCM_SetCommitIndexNotifiesChangeListener(t *testing.T) {
	f := func(setup func(t *testing.T) (mcm *managedConsensusModule, mrs *testhelpers.MockRpcSender)) {
		mcm, _ := setup(t)

		if mcm.dsm.GetCommitIndex() != 0 {
			t.Fatal()
		}

		err := mcm.pcm.setCommitIndex(2)
		if err != nil {
			t.Fatal(err)
		}
		if mcm.dsm.GetCommitIndex() != 2 {
			t.Fatal()
		}

		err = mcm.pcm.setCommitIndex(9)
		if err != nil {
			t.Fatal(err)
		}
		if mcm.dsm.GetCommitIndex() != 9 {
			t.Fatal()
		}
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
	err := mcm.pcm.RpcReply_RpcRequestVoteReply("s2", sentRpc, &RpcRequestVoteReply{serverTerm, true})
	if err != nil {
		t.Fatal(err)
	}
	err = mcm.pcm.RpcReply_RpcRequestVoteReply("s3", sentRpc, &RpcRequestVoteReply{serverTerm, true})
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
) (*managedConsensusModule, *testhelpers.MockRpcSender) {
	mcm, mrs := testSetupMCM_SOLO_Follower_WithTerms(t, terms)
	testCM_SOLO_Follower_ElectsSelfOnElectionTimeout(t, mcm, mrs)
	if mcm.pcm.GetServerState() != LEADER {
		t.Fatal()
	}
	return mcm, mrs
}

func testSetupMCM_Follower_Figure7LeaderLine(t *testing.T) (*managedConsensusModule, *testhelpers.MockRpcSender) {
	return testSetupMCM_Follower_WithTerms(t, testdata.TestUtil_MakeFigure7LeaderLineTerms())
}

func testSetupMCM_Candidate_Figure7LeaderLine(t *testing.T) (*managedConsensusModule, *testhelpers.MockRpcSender) {
	return testSetupMCM_Candidate_WithTerms(t, testdata.TestUtil_MakeFigure7LeaderLineTerms())
}

func testSetupMCM_Leader_Figure7LeaderLine(t *testing.T) (*managedConsensusModule, *testhelpers.MockRpcSender) {
	return testSetupMCM_Leader_WithTerms(t, testdata.TestUtil_MakeFigure7LeaderLineTerms())
}

func testSetupMCM_Leader_Figure7LeaderLine_WithUpToDatePeers(t *testing.T) (*managedConsensusModule, *testhelpers.MockRpcSender) {
	mcm, mrs := testSetupMCM_Leader_WithTerms(t, testdata.TestUtil_MakeFigure7LeaderLineTerms())

	// sanity check - before
	expectedNextIndex := map[ServerId]LogIndex{"s2": 11, "s3": 11, "s4": 11, "s5": 11}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.NextIndex, expectedNextIndex) {
		t.Fatal()
	}
	expectedMatchIndex := map[ServerId]LogIndex{"s2": 0, "s3": 0, "s4": 0, "s5": 0}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.MatchIndex, expectedMatchIndex) {
		t.Fatal()
	}

	// pretend peers caught up!
	lastLogIndex, err := mcm.pcm.LogRO.GetIndexOfLastEntry()
	if err != nil {
		t.Fatal()
	}
	err = mcm.pcm.ClusterInfo.ForEachPeer(
		func(serverId ServerId) error {
			err := mcm.pcm.LeaderVolatileState.SetMatchIndexAndNextIndex(serverId, lastLogIndex)
			if err != nil {
				return err
			}
			return nil
		},
	)
	if err != nil {
		t.Fatal()
	}

	// after check
	expectedNextIndex = map[ServerId]LogIndex{"s2": 11, "s3": 11, "s4": 11, "s5": 11}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.NextIndex, expectedNextIndex) {
		t.Fatal()
	}
	expectedMatchIndex = map[ServerId]LogIndex{"s2": 10, "s3": 10, "s4": 10, "s5": 10}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.MatchIndex, expectedMatchIndex) {
		t.Fatal()
	}

	return mcm, mrs
}

func TestCM_Follower_StartsElectionOnElectionTimeout_NonEmptyLog(t *testing.T) {
	testSetupMCM_Candidate_Figure7LeaderLine(t)
}

// #RFS-L2a: If Command received from client: append entry to local log
func TestCM_Leader_AppendCommand(t *testing.T) {
	mcm, _ := testSetupMCM_Leader_Figure7LeaderLine(t)

	// pre check
	iole, err := mcm.pcm.LogRO.GetIndexOfLastEntry()
	if err != nil {
		t.Fatal()
	}
	if iole != 10 {
		t.Fatal()
	}

	ioleAC, err := mcm.pcm.AppendCommand(testhelpers.DummyCommand(1101))
	if err != nil || ioleAC != 11 {
		t.Fatal()
	}

	iole, err = mcm.pcm.LogRO.GetIndexOfLastEntry()
	if err != nil {
		t.Fatal()
	}
	if iole != 11 {
		t.Fatal()
	}
	le := testhelpers.TestHelper_GetLogEntryAtIndex(mcm.pcm.LogRO, 11)
	if !reflect.DeepEqual(le, LogEntry{8, Command("c1101")}) {
		t.Fatal(le)
	}
}

// #RFS-L2a: If Command received from client: append entry to local log
func TestCM_FollowerOrCandidate_AppendCommand(t *testing.T) {
	f := func(
		setup func(t *testing.T) (mcm *managedConsensusModule, mrs *testhelpers.MockRpcSender),
	) {
		mcm, _ := setup(t)

		// pre check
		iole, err := mcm.pcm.LogRO.GetIndexOfLastEntry()
		if err != nil {
			t.Fatal()
		}
		if iole != 10 {
			t.Fatal()
		}

		_, err = mcm.pcm.AppendCommand(testhelpers.DummyCommand(1101))
		if err != ErrNotLeader {
			t.Fatal()
		}

		iole, err = mcm.pcm.LogRO.GetIndexOfLastEntry()
		if err != nil {
			t.Fatal()
		}
		if iole != 10 {
			t.Fatal()
		}
	}

	f(testSetupMCM_Follower_Figure7LeaderLine)
	f(testSetupMCM_Candidate_Figure7LeaderLine)
}

// For most tests, we'll use a passive CM where we control the progress
// of time with helper methods. This simplifies tests and avoids concurrency
// issues with inspecting the internals.
type managedConsensusModule struct {
	pcm  *PassiveConsensusModule
	now  time.Time
	diml LogReadOnly
	dsm  *testhelpers.DummyStateMachine
}

func (mcm *managedConsensusModule) Tick() error {
	err := mcm.pcm.Tick(mcm.now)
	if err != nil {
		return err
	}
	mcm.now = mcm.now.Add(testdata.TickerDuration)
	return nil
}

func (mcm *managedConsensusModule) Rpc_RpcAppendEntries(
	from ServerId,
	rpc *RpcAppendEntries,
) (*RpcAppendEntriesReply, error) {
	return mcm.pcm.Rpc_RpcAppendEntries(from, rpc, mcm.now)
}

func (mcm *managedConsensusModule) Rpc_RpcRequestVote(
	from ServerId,
	rpc *RpcRequestVote,
) (*RpcRequestVoteReply, error) {
	return mcm.pcm.Rpc_RpcRequestVote(from, rpc, mcm.now)
}

func (mcm *managedConsensusModule) tickTilElectionTimeout(t *testing.T) {
	electionTimeoutTime := mcm.pcm.ElectionTimeoutTracker.GetElectionTimeoutTime()
	for {
		err := mcm.Tick()
		if err != nil {
			t.Fatal(err)
		}
		if mcm.now.After(electionTimeoutTime) {
			break
		}
	}
	if mcm.pcm.ElectionTimeoutTracker.GetElectionTimeoutTime() != electionTimeoutTime {
		t.Fatal("electionTimeoutTime changed!")
	}
	// Because tick() increments "now" after calling tick(),
	// we need one more to actually run with a post-timeout "now".
	err := mcm.Tick()
	if err != nil {
		t.Fatal(err)
	}
}
