package impl

import (
	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/config"
	"github.com/divtxt/raft/consensus"
	"github.com/divtxt/raft/log"
	"github.com/divtxt/raft/rps"
	"github.com/divtxt/raft/testdata"
	"github.com/divtxt/raft/testhelpers"
	"reflect"
	"testing"
	"time"
)

func makeAEWithTerm(term TermNo) *RpcAppendEntries {
	return &RpcAppendEntries{term, 0, 0, nil, 0}
}

func setupConsensusModule(t *testing.T, logTerms []TermNo) *ConsensusModule {
	cm, _ := setupConsensusModuleR2(t, logTerms)
	return cm
}

func setupConsensusModuleR2(
	t *testing.T,
	logTerms []TermNo,
) (*ConsensusModule, *testhelpers.MockRpcSender) {
	ps := rps.NewIMPSWithCurrentTerm(testdata.CurrentTerm)
	iml := log.TestUtil_NewInMemoryLog_WithTerms(logTerms)
	dsm := testhelpers.NewDummyStateMachine()
	mrs := testhelpers.NewMockRpcSender()
	ts := config.TimeSettings{testdata.TickerDuration, testdata.ElectionTimeoutLow}
	ci, err := config.NewClusterInfo(testdata.AllServerIds, testdata.ThisServerId)
	if err != nil {
		t.Fatal(err)
	}
	cm, err := NewConsensusModule(ps, iml, mrs, ci, testdata.MaxEntriesPerAppendEntry, ts)
	if err != nil {
		t.Fatal(err)
	}
	if cm == nil {
		t.Fatal()
	}
	err = cm.Start(dsm)
	if err != nil {
		t.Fatal(err)
	}
	return cm, mrs
}

func TestConsensusModule_StartStateAndStop(t *testing.T) {
	cm := setupConsensusModule(t, nil)

	// #5.2-p1s2: When servers start up, they begin as followers
	if cm.GetServerState() != FOLLOWER {
		t.Fatal()
	}

	time.Sleep(testdata.SleepJustMoreThanATick)

	if cm.IsStopped() {
		t.Error()
	}

	cm.Stop()

	if !cm.IsStopped() {
		t.Error()
	}
}

func TestConsensusModule_CallStopMultipleTimes(t *testing.T) {
	cm := setupConsensusModule(t, nil)

	time.Sleep(testdata.SleepJustMoreThanATick)
	if cm.IsStopped() {
		t.Error()
	}

	cm.Stop()

	if !cm.IsStopped() {
		t.Error()
	}

	cm.Stop()
}

func TestConsensusModule_ProcessRpcAppendEntries(t *testing.T) {
	cm := setupConsensusModule(t, nil)
	defer cm.Stop()
	var serverTerm TermNo = testdata.CurrentTerm

	reply := cm.ProcessRpcAppendEntries("s2", makeAEWithTerm(serverTerm-1))

	expectedRpc := RpcAppendEntriesReply{serverTerm, false}
	if reply == nil || *reply != expectedRpc {
		t.Fatal(reply)
	}
}

func TestConsensusModule_ProcessRpcAppendEntries_StoppedCM(t *testing.T) {
	cm := setupConsensusModule(t, nil)
	cm.Stop()

	reply := cm.ProcessRpcAppendEntries("s2", makeAEWithTerm(testdata.CurrentTerm-1))

	if reply != nil {
		t.Fatal(reply)
	}
}

func TestConsensusModule_ProcessRpcRequestVote(t *testing.T) {
	cm := setupConsensusModule(t, nil)
	defer cm.Stop()

	reply := cm.ProcessRpcRequestVote("s2", &RpcRequestVote{testdata.CurrentTerm - 1, 0, 0})

	serverTerm := cm.passiveConsensusModule.RaftPersistentState.GetCurrentTerm()
	expectedRpc := RpcRequestVoteReply{serverTerm, false}
	if reply == nil || *reply != expectedRpc {
		t.Fatal(reply)
	}
}

func TestConsensusModule_ProcessRpcRequestVote_StoppedCM(t *testing.T) {
	cm := setupConsensusModule(t, nil)
	cm.Stop()

	reply := cm.ProcessRpcRequestVote(
		"s2",
		&RpcRequestVote{testdata.CurrentTerm - 1, 0, 0},
	)

	if reply != nil {
		t.Fatal(reply)
	}
}

// Run through an election cycle to test the rpc reply callbacks!
func TestConsensusModule_RpcReplyCallbackFunction(t *testing.T) {
	cm, mrs := setupConsensusModuleR2(t, nil)
	defer cm.Stop()

	testConsensusModule_RpcReplyCallback_AndBecomeLeader(t, cm, mrs)
}

func testConsensusModule_RpcReplyCallback_AndBecomeLeader(
	t *testing.T,
	cm *ConsensusModule,
	mrs *testhelpers.MockRpcSender,
) {
	// FIXME: multiple unsafe concurrent accesses

	// Sleep till election starts
	ett := cm.passiveConsensusModule.ElectionTimeoutTracker
	max_ticks := (ett.GetCurrentElectionTimeout().Nanoseconds() / testdata.TickerDuration.Nanoseconds()) + 2
	for i := int64(0); i < max_ticks; i++ {
		time.Sleep(testdata.TickerDuration)
		if cm.GetServerState() != FOLLOWER {
			break
		}
	}

	if cm.GetServerState() != CANDIDATE {
		t.Fatal()
	}

	// candidate has issued RequestVote RPCs to all other servers.
	lastLogIndex, lastLogTerm, err := consensus.GetIndexAndTermOfLastEntry(cm.passiveConsensusModule.LogRO)
	if err != nil {
		t.Fatal(err)
	}
	expectedRpc := &RpcRequestVote{testdata.CurrentTerm + 1, lastLogIndex, lastLogTerm}
	expectedRpcs := map[ServerId]interface{}{
		"s2": expectedRpc,
		"s3": expectedRpc,
		"s4": expectedRpc,
		"s5": expectedRpc,
	}
	mrs.CheckSentRpcs(t, expectedRpcs)

	// reply true for all votes
	serverTerm := cm.passiveConsensusModule.RaftPersistentState.GetCurrentTerm()
	if n := mrs.SendRVRepliesAndClearRpcs(&RpcRequestVoteReply{serverTerm, true}); n != 4 {
		t.Fatal(n)
	}

	time.Sleep(testdata.SleepToLetGoroutineRun)

	// server should now be a leader
	if cm.IsStopped() {
		t.Fatal()
	}
	if cm.GetServerState() != LEADER {
		t.Fatal()
	}

	// leader setup
	expectedRpc2 := &RpcAppendEntries{
		serverTerm,
		lastLogIndex,
		lastLogTerm,
		[]LogEntry{},
		cm.passiveConsensusModule.GetCommitIndex(),
	}
	expectedRpcs2 := map[ServerId]interface{}{
		"s2": expectedRpc2,
		"s3": expectedRpc2,
		"s4": expectedRpc2,
		"s5": expectedRpc2,
	}
	mrs.CheckSentRpcs(t, expectedRpcs2)

	// reply handling
	expectedMatchIndex := map[ServerId]LogIndex{"s2": 0, "s3": 0, "s4": 0, "s5": 0}
	if !reflect.DeepEqual(cm.passiveConsensusModule.LeaderVolatileState.MatchIndex, expectedMatchIndex) {
		t.Fatal()
	}

	if n := mrs.SendAERepliesAndClearRpcs(&RpcAppendEntriesReply{serverTerm, true}); n != 4 {
		t.Fatal(n)
	}

	time.Sleep(testdata.SleepToLetGoroutineRun)

	expectedMatchIndex = map[ServerId]LogIndex{
		"s2": lastLogIndex,
		"s3": lastLogIndex,
		"s4": lastLogIndex,
		"s5": lastLogIndex,
	}
	if !reflect.DeepEqual(cm.passiveConsensusModule.LeaderVolatileState.MatchIndex, expectedMatchIndex) {
		t.Fatal(cm.passiveConsensusModule.LeaderVolatileState.MatchIndex)
	}
}

func TestConsensusModule_AppendCommand_Leader(t *testing.T) {
	cm, mrs := setupConsensusModuleR2(t, testdata.TestUtil_MakeFigure7LeaderLineTerms())
	defer cm.Stop()

	testConsensusModule_RpcReplyCallback_AndBecomeLeader(t, cm, mrs)

	// pre check
	iole, err := cm.passiveConsensusModule.LogRO.GetIndexOfLastEntry()
	if err != nil {
		t.Fatal()
	}
	if iole != 10 {
		t.Fatal()
	}

	ioleAC, err := cm.AppendCommand(testhelpers.DummyCommand(1101))

	if cm.IsStopped() {
		t.Error()
	}
	if err != nil {
		t.Fatal()
	}
	if ioleAC != 11 {
		t.Fatal()
	}

	iole, err = cm.passiveConsensusModule.LogRO.GetIndexOfLastEntry()
	if err != nil {
		t.Fatal()
	}
	if iole != 11 {
		t.Fatal()
	}
	le := testhelpers.TestHelper_GetLogEntryAtIndex(cm.passiveConsensusModule.LogRO, 11)
	if !reflect.DeepEqual(le, LogEntry{8, Command("c1101")}) {
		t.Fatal(le)
	}
}

func TestConsensusModule_AppendCommand_Follower(t *testing.T) {
	cm, _ := setupConsensusModuleR2(t, testdata.TestUtil_MakeFigure7LeaderLineTerms())
	defer cm.Stop()

	// pre check
	iole, err := cm.passiveConsensusModule.LogRO.GetIndexOfLastEntry()
	if err != nil {
		t.Fatal()
	}
	if iole != 10 {
		t.Fatal()
	}

	_, err = cm.AppendCommand(testhelpers.DummyCommand(1101))

	if err != ErrNotLeader {
		t.Fatal()
	}
	if cm.IsStopped() {
		t.Error()
	}

	iole, err = cm.passiveConsensusModule.LogRO.GetIndexOfLastEntry()
	if err != nil {
		t.Fatal()
	}
	if iole != 10 {
		t.Fatal()
	}
}

func TestConsensusModule_AppendCommand_Follower_StoppedCM(t *testing.T) {
	cm, _ := setupConsensusModuleR2(t, testdata.TestUtil_MakeFigure7LeaderLineTerms())
	cm.Stop()

	_, err := cm.AppendCommand(testhelpers.DummyCommand(1101))

	if err != ErrStopped {
		t.Fatal(err)
	}
}
