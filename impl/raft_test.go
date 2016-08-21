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
	iml := log.TestUtil_NewInMemoryLog_WithTerms(logTerms, testdata.MaxEntriesPerAppendEntry)
	dsm := testhelpers.NewDummyStateMachine()
	mrs := testhelpers.NewMockRpcSender()
	ts := config.TimeSettings{testdata.TickerDuration, testdata.ElectionTimeoutLow}
	ci, err := config.NewClusterInfo(testdata.AllServerIds, testdata.ThisServerId)
	if err != nil {
		t.Fatal(err)
	}
	cm, err := NewConsensusModule(ps, iml, dsm, mrs, ci, ts)
	if err != nil {
		t.Fatal(err)
	}
	if cm == nil {
		t.Fatal()
	}
	return cm, mrs
}

func (cm *ConsensusModule) stopAsyncWithRecover() (e interface{}) {
	defer func() {
		e = recover()
	}()
	cm.StopAsync()
	return nil
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

	cm.StopAsync()
	time.Sleep(testdata.SleepToLetGoroutineRun)

	if !cm.IsStopped() {
		t.Error()
	}
	if cm.GetStopError() != nil {
		t.Error()
	}
}

func TestConsensusModule_CallStopAsyncMultipleTimes(t *testing.T) {
	cm := setupConsensusModule(t, nil)

	time.Sleep(testdata.SleepJustMoreThanATick)
	if cm.IsStopped() {
		t.Error()
	}

	cm.StopAsync()
	cm.StopAsync()
	time.Sleep(testdata.SleepToLetGoroutineRun)

	if !cm.IsStopped() {
		t.Error()
	}
	if cm.GetStopError() != nil {
		t.Error()
	}

	cm.StopAsync()
}

func TestConsensusModule_ProcessRpcAppendEntriesAsync(t *testing.T) {
	cm := setupConsensusModule(t, nil)
	defer cm.StopAsync()
	var serverTerm TermNo = testdata.CurrentTerm

	replyChan := cm.ProcessRpcAppendEntriesAsync("s2", makeAEWithTerm(serverTerm-1))
	time.Sleep(testdata.SleepToLetGoroutineRun)

	select {
	case reply := <-replyChan:
		expectedRpc := RpcAppendEntriesReply{serverTerm, false}
		if *reply != expectedRpc {
			t.Fatal(reply)
		}
	default:
		t.Fatal()
	}
}

func TestConsensusModule_ProcessRpcAppendEntriesAsync_StoppedCM(t *testing.T) {
	cm := setupConsensusModule(t, nil)
	cm.StopAsync()
	time.Sleep(testdata.SleepToLetGoroutineRun)

	replyChan := cm.ProcessRpcAppendEntriesAsync("s2", makeAEWithTerm(testdata.CurrentTerm-1))
	time.Sleep(testdata.SleepToLetGoroutineRun)

	select {
	case <-replyChan:
		t.Fatal()
	default:
	}
}

func TestConsensusModule_ProcessRpcRequestVoteAsync(t *testing.T) {
	cm := setupConsensusModule(t, nil)
	defer cm.StopAsync()

	replyChan := cm.ProcessRpcRequestVoteAsync("s2", &RpcRequestVote{testdata.CurrentTerm - 1, 0, 0})
	time.Sleep(testdata.SleepToLetGoroutineRun)

	select {
	case reply := <-replyChan:
		serverTerm := cm.passiveConsensusModule.RaftPersistentState.GetCurrentTerm()
		expectedRpc := RpcRequestVoteReply{serverTerm, false}
		if *reply != expectedRpc {
			t.Fatal(reply)
		}
	default:
		t.Fatal()
	}
}

func TestConsensusModule_ProcessRpcRequestVoteAsync_StoppedCM(t *testing.T) {
	cm := setupConsensusModule(t, nil)
	cm.StopAsync()
	time.Sleep(testdata.SleepToLetGoroutineRun)

	replyChan := cm.ProcessRpcRequestVoteAsync(
		"s2",
		&RpcRequestVote{testdata.CurrentTerm - 1, 0, 0},
	)
	time.Sleep(testdata.SleepToLetGoroutineRun)

	select {
	case <-replyChan:
		t.Fatal()
	default:
	}
}

// Run through an election cycle to test the rpc reply callbacks!
func TestConsensusModule_RpcReplyCallbackFunction(t *testing.T) {
	cm, mrs := setupConsensusModuleR2(t, nil)
	defer cm.StopAsync()

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
	expectedRpcs := []testhelpers.MockSentRpc{
		{"s2", expectedRpc},
		{"s3", expectedRpc},
		{"s4", expectedRpc},
		{"s5", expectedRpc},
	}
	mrs.CheckSentRpcs(t, expectedRpcs)

	// reply true for all votes
	serverTerm := cm.passiveConsensusModule.RaftPersistentState.GetCurrentTerm()
	if mrs.SendReplies(&RpcRequestVoteReply{serverTerm, true}) != 4 {
		t.Fatal()
	}

	time.Sleep(testdata.SleepToLetGoroutineRun)

	// server should now be a leader
	if cm.IsStopped() {
		t.Fatal(cm.GetStopError())
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
	expectedRpcs2 := []testhelpers.MockSentRpc{
		{"s2", expectedRpc2},
		{"s3", expectedRpc2},
		{"s4", expectedRpc2},
		{"s5", expectedRpc2},
	}
	mrs.CheckSentRpcs(t, expectedRpcs2)

	// reply handling
	expectedMatchIndex := map[ServerId]LogIndex{"s2": 0, "s3": 0, "s4": 0, "s5": 0}
	if !reflect.DeepEqual(cm.passiveConsensusModule.LeaderVolatileState.MatchIndex, expectedMatchIndex) {
		t.Fatal()
	}

	if mrs.SendReplies(&RpcAppendEntriesReply{serverTerm, true}) != 4 {
		t.Fatal()
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

func TestConsensusModule_AppendCommandAsync_Leader(t *testing.T) {
	cm, mrs := setupConsensusModuleR2(t, testdata.TestUtil_MakeFigure7LeaderLineTerms())
	defer cm.StopAsync()

	testConsensusModule_RpcReplyCallback_AndBecomeLeader(t, cm, mrs)

	// pre check
	iole, err := cm.passiveConsensusModule.LogRO.GetIndexOfLastEntry()
	if err != nil {
		t.Fatal()
	}
	if iole != 10 {
		t.Fatal()
	}

	replyChan := cm.AppendCommandAsync(testhelpers.DummyCommand(1101))

	iole, err = cm.passiveConsensusModule.LogRO.GetIndexOfLastEntry()
	if err != nil {
		t.Fatal()
	}
	if iole != 10 {
		t.Fatal()
	}

	time.Sleep(testdata.SleepToLetGoroutineRun)

	select {
	case result := <-replyChan:
		if cm.IsStopped() {
			t.Error(cm.GetStopError())
		}
		if result != nil {
			t.Fatal()
		}
		iole, err = cm.passiveConsensusModule.LogRO.GetIndexOfLastEntry()
		if err != nil {
			t.Fatal()
		}
		if iole != 11 {
			t.Fatal()
		}
		le := log.TestHelper_GetLogEntryAtIndex(cm.passiveConsensusModule.LogRO, 11)
		if !reflect.DeepEqual(le, LogEntry{8, Command("c1101")}) {
			t.Fatal(le)
		}
	default:
		t.Fatal()
	}
}

func TestConsensusModule_AppendCommandAsync_Follower(t *testing.T) {
	cm, _ := setupConsensusModuleR2(t, testdata.TestUtil_MakeFigure7LeaderLineTerms())
	defer cm.StopAsync()

	// pre check
	iole, err := cm.passiveConsensusModule.LogRO.GetIndexOfLastEntry()
	if err != nil {
		t.Fatal()
	}
	if iole != 10 {
		t.Fatal()
	}

	replyChan := cm.AppendCommandAsync(testhelpers.DummyCommand(1101))

	iole, err = cm.passiveConsensusModule.LogRO.GetIndexOfLastEntry()
	if err != nil {
		t.Fatal()
	}
	if iole != 10 {
		t.Fatal()
	}

	time.Sleep(testdata.SleepToLetGoroutineRun)

	select {
	case result := <-replyChan:
		if cm.IsStopped() {
			t.Error(cm.GetStopError())
		}
		if result != ErrNotLeader {
			t.Fatal()
		}
		iole, err := cm.passiveConsensusModule.LogRO.GetIndexOfLastEntry()
		if err != nil {
			t.Fatal()
		}
		if iole != 10 {
			t.Fatal()
		}
	default:
		t.Fatal()
	}
}

func TestConsensusModule_AppendCommandAsync_Follower_StoppedCM(t *testing.T) {
	cm, _ := setupConsensusModuleR2(t, testdata.TestUtil_MakeFigure7LeaderLineTerms())
	cm.StopAsync()
	time.Sleep(testdata.SleepToLetGoroutineRun)

	replyChan := cm.AppendCommandAsync(testhelpers.DummyCommand(1101))
	time.Sleep(testdata.SleepToLetGoroutineRun)

	select {
	case <-replyChan:
		t.Fatal()
	default:
	}
}
