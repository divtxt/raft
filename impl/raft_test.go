package impl

import (
	"log"
	"os"
	"reflect"
	"testing"
	"time"

	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/config"
	"github.com/divtxt/raft/consensus"
	"github.com/divtxt/raft/inmemlog"
	"github.com/divtxt/raft/rps"
	"github.com/divtxt/raft/testdata"
	"github.com/divtxt/raft/testhelpers"
)

func makeAEWithTerm(term TermNo) *RpcAppendEntries {
	return &RpcAppendEntries{term, 0, 0, nil, 0}
}

func setupConsensusModule(t *testing.T) *ConsensusModule {
	cm, _, _ := setupConsensusModuleR2(t, nil, 0)
	return cm
}

func setupConsensusModuleR2(
	t *testing.T,
	logTerms []TermNo,
	discardEntriesBeforeIndex LogIndex,
) (*ConsensusModule, *testhelpers.MockRpcSender, Log) {
	ps := rps.NewIMPSWithCurrentTerm(testdata.CurrentTerm)

	iml, err := inmemlog.TestUtil_NewInMemoryLog_WithTerms(
		logTerms, testdata.MaxEntriesPerAppendEntry,
	)
	if err != nil {
		t.Fatal(err)
	}
	if discardEntriesBeforeIndex > 0 {
		err := iml.DiscardEntriesBeforeIndex(discardEntriesBeforeIndex)
		if err != nil {
			t.Fatal(err)
		}
	}

	dsm := testhelpers.NewDummyStateMachine(0) // FIXME: test with non-zero value
	mrs := testhelpers.NewMockRpcSender()
	ts := config.TimeSettings{testdata.TickerDuration, testdata.ElectionTimeoutLow}
	ci, err := config.NewClusterInfo(testdata.AllServerIds, testdata.ThisServerId)
	if err != nil {
		t.Fatal(err)
	}
	logger := log.New(os.Stderr, "integration_test", log.Flags())
	cm, err := NewConsensusModule(ps, iml, dsm, mrs, ci, ts, logger)
	if err != nil {
		t.Fatal(err)
	}
	if cm == nil {
		t.Fatal()
	}
	return cm, mrs, iml
}

func TestConsensusModule_StartStateAndStop(t *testing.T) {
	cm := setupConsensusModule(t)

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
	cm := setupConsensusModule(t)

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
	cm := setupConsensusModule(t)
	defer cm.Stop()
	var serverTerm TermNo = testdata.CurrentTerm

	reply, err := cm.ProcessRpcAppendEntries(102, makeAEWithTerm(serverTerm-1))
	if err != nil {
		t.Fatal(err)
	}

	expectedRpc := RpcAppendEntriesReply{serverTerm, false}
	if reply == nil || *reply != expectedRpc {
		t.Fatal(reply)
	}
}

func TestConsensusModule_ProcessRpcAppendEntries_StoppedCM(t *testing.T) {
	cm := setupConsensusModule(t)
	cm.Stop()

	_, err := cm.ProcessRpcAppendEntries(102, makeAEWithTerm(testdata.CurrentTerm-1))

	if err != ErrStopped {
		t.Fatal(err)
	}
}

func TestConsensusModule_ProcessRpcRequestVote(t *testing.T) {
	cm := setupConsensusModule(t)
	defer cm.Stop()

	reply, err := cm.ProcessRpcRequestVote(102, &RpcRequestVote{testdata.CurrentTerm - 1, 0, 0})
	if err != nil {
		t.Fatal(err)
	}

	serverTerm := cm.passiveConsensusModule.RaftPersistentState.GetCurrentTerm()
	expectedRpc := RpcRequestVoteReply{serverTerm, false}
	if reply == nil || *reply != expectedRpc {
		t.Fatal(reply)
	}
}

func TestConsensusModule_ProcessRpcRequestVote_StoppedCM(t *testing.T) {
	cm := setupConsensusModule(t)
	cm.Stop()

	_, err := cm.ProcessRpcRequestVote(
		102,
		&RpcRequestVote{testdata.CurrentTerm - 1, 0, 0},
	)

	if err != ErrStopped {
		t.Fatal(err)
	}
}

// Run through an election cycle to test the rpc reply callbacks!
func TestConsensusModule_RpcReplyCallbackFunction(t *testing.T) {
	cm, mrs, log := setupConsensusModuleR2(t, nil, 0)
	defer cm.Stop()

	testConsensusModule_RpcReplyCallback_AndBecomeLeader(t, cm, mrs, log)
}

func testConsensusModule_RpcReplyCallback_AndBecomeLeader(
	t *testing.T,
	cm *ConsensusModule,
	mrs *testhelpers.MockRpcSender,
	log Log,
) {
	// FIXME: multiple unsafe concurrent accesses

	// Sleep till election starts
	ett := cm.passiveConsensusModule.ElectionTimeoutTimer
	max_ticks := (ett.GetCurrentDuration().Nanoseconds() / testdata.TickerDuration.Nanoseconds()) + 2
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
	lastLogIndex, lastLogTerm, err := consensus.GetIndexAndTermOfLastEntry(log)
	if err != nil {
		t.Fatal(err)
	}
	expectedRpc := &RpcRequestVote{testdata.CurrentTerm + 1, lastLogIndex, lastLogTerm}
	expectedRpcs := map[ServerId]interface{}{
		102: expectedRpc,
		103: expectedRpc,
		104: expectedRpc,
		105: expectedRpc,
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
		102: expectedRpc2,
		103: expectedRpc2,
		104: expectedRpc2,
		105: expectedRpc2,
	}
	mrs.CheckSentRpcs(t, expectedRpcs2)

	// reply handling
	expectedMatchIndex := map[ServerId]LogIndex{102: 0, 103: 0, 104: 0, 105: 0}
	if !reflect.DeepEqual(
		cm.passiveConsensusModule.LeaderVolatileState.MatchIndexes(),
		expectedMatchIndex,
	) {
		t.Fatal()
	}

	if n := mrs.SendAERepliesAndClearRpcs(&RpcAppendEntriesReply{serverTerm, true}); n != 4 {
		t.Fatal(n)
	}

	time.Sleep(testdata.SleepToLetGoroutineRun)

	expectedMatchIndex = map[ServerId]LogIndex{
		102: lastLogIndex,
		103: lastLogIndex,
		104: lastLogIndex,
		105: lastLogIndex,
	}
	if !reflect.DeepEqual(
		cm.passiveConsensusModule.LeaderVolatileState.MatchIndexes(),
		expectedMatchIndex,
	) {
		t.Fatal(cm.passiveConsensusModule.LeaderVolatileState.MatchIndexes())
	}
}

func TestConsensusModule_AppendCommand_Leader(t *testing.T) {
	cm, mrs, log := setupConsensusModuleR2(
		t, testdata.TestUtil_MakeFigure7LeaderLineTerms(), 0,
	)
	defer cm.Stop()

	testConsensusModule_RpcReplyCallback_AndBecomeLeader(t, cm, mrs, log)

	// pre check
	iole := log.GetIndexOfLastEntry()
	if iole != 10 {
		t.Fatal()
	}

	crc1101, err := cm.AppendCommand(testhelpers.DummyCommand(1101))

	if cm.IsStopped() {
		t.Error()
	}
	if err != nil {
		t.Fatal()
	}
	if crc1101 == nil {
		t.Fatal()
	}
	testhelpers.AssertWillBlock(crc1101)

	iole = log.GetIndexOfLastEntry()
	if iole != 11 {
		t.Fatal()
	}
	le := testhelpers.TestHelper_GetLogEntryAtIndex(log, 11)
	if !reflect.DeepEqual(le, LogEntry{8, Command("c1101")}) {
		t.Fatal(le)
	}
}

func TestConsensusModule_AppendCommand_Follower(t *testing.T) {
	cm, _, log := setupConsensusModuleR2(
		t, testdata.TestUtil_MakeFigure7LeaderLineTerms(), 0,
	)
	defer cm.Stop()

	// pre check
	iole := log.GetIndexOfLastEntry()
	if iole != 10 {
		t.Fatal()
	}

	_, err := cm.AppendCommand(testhelpers.DummyCommand(1101))

	if err != ErrNotLeader {
		t.Fatal()
	}
	if cm.IsStopped() {
		t.Error()
	}

	iole = log.GetIndexOfLastEntry()
	if iole != 10 {
		t.Fatal()
	}
}

func TestConsensusModule_AppendCommand_Follower_StoppedCM(t *testing.T) {
	cm, _, _ := setupConsensusModuleR2(
		t, testdata.TestUtil_MakeFigure7LeaderLineTerms(), 0,
	)

	cm.Stop()

	_, err := cm.AppendCommand(testhelpers.DummyCommand(1101))

	if err != ErrStopped {
		t.Fatal(err)
	}
}
