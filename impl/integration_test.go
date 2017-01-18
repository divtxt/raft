package impl

import (
	"reflect"
	"testing"
	"time"

	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/config"
	"github.com/divtxt/raft/log"
	"github.com/divtxt/raft/rps"
	"github.com/divtxt/raft/testdata"
	"github.com/divtxt/raft/testhelpers"
)

var testClusterServerIds = []ServerId{101, 102, 103}

func setupConsensusModuleR3(
	t *testing.T,
	thisServerId ServerId,
	electionTimeoutLow time.Duration,
	logTerms []TermNo,
	imrsc *inMemoryRpcServiceConnector,
) (IConsensusModule, *log.InMemoryLog, *testhelpers.DummyStateMachine) {
	ps := rps.NewIMPSWithCurrentTerm(0)
	iml := log.TestUtil_NewInMemoryLog_WithTerms(logTerms)
	dsm := testhelpers.NewDummyStateMachine(0) // FIXME: test with non-zero value
	ts := config.TimeSettings{testdata.TickerDuration, electionTimeoutLow}
	ci, err := config.NewClusterInfo(testClusterServerIds, thisServerId)
	if err != nil {
		t.Fatal(err)
	}
	cm, err := NewConsensusModule(ps, iml, dsm, imrsc, ci, testdata.MaxEntriesPerAppendEntry, ts)
	if err != nil {
		t.Fatal(err)
	}
	if cm == nil {
		t.Fatal()
	}
	return cm, iml, dsm
}

func setupConsensusModuleR3_SOLO(
	t *testing.T,
	electionTimeoutLow time.Duration,
	logTerms []TermNo,
	imrsc *inMemoryRpcServiceConnector,
) (IConsensusModule, *log.InMemoryLog, *testhelpers.DummyStateMachine) {
	ps := rps.NewIMPSWithCurrentTerm(0)
	iml := log.TestUtil_NewInMemoryLog_WithTerms(logTerms)
	dsm := testhelpers.NewDummyStateMachine(0) // FIXME: test with non-zero value
	ts := config.TimeSettings{testdata.TickerDuration, testdata.ElectionTimeoutLow}
	ci, err := config.NewClusterInfo([]ServerId{101}, 101)
	if err != nil {
		t.Fatal(err)
	}
	cm, err := NewConsensusModule(ps, iml, dsm, imrsc, ci, testdata.MaxEntriesPerAppendEntry, ts)
	if err != nil {
		t.Fatal(err)
	}
	if cm == nil {
		t.Fatal()
	}
	return cm, iml, dsm
}

func TestCluster_ElectsLeader(t *testing.T) {
	imrsh := &inMemoryRpcServiceHub{nil}
	setupCMR3 := func(thisServerId ServerId) IConsensusModule {
		cm, _, _ := setupConsensusModuleR3(
			t,
			thisServerId,
			testdata.ElectionTimeoutLow,
			nil,
			imrsh.getRpcService(thisServerId),
		)
		return cm
	}
	cm1 := setupCMR3(101)
	defer cm1.Stop()
	cm2 := setupCMR3(102)
	defer cm2.Stop()
	cm3 := setupCMR3(103)
	defer cm3.Stop()
	imrsh.cms = map[ServerId]IConsensusModule{
		101: cm1,
		102: cm2,
		103: cm3,
	}

	// -- All nodes start as followers
	totalState := cm1.GetServerState() + cm2.GetServerState() + cm3.GetServerState()
	if totalState != 0 {
		t.Fatal(totalState)
	}

	time.Sleep(testdata.ElectionTimeoutLow - testdata.SleepToLetGoroutineRun)

	totalState = cm1.GetServerState() + cm2.GetServerState() + cm3.GetServerState()
	if totalState != 0 {
		t.Fatal(totalState)
	}

	// -- Election timeout results in a leader being elected
	// (note: this test has a low probability race condition where two nodes
	// can become candidates at the same time and no leader is elected)
	time.Sleep(
		testdata.SleepToLetGoroutineRun + testdata.ElectionTimeoutLow + testdata.SleepJustMoreThanATick,
	)

	totalState = cm1.GetServerState()*100 + cm2.GetServerState()*10 + cm3.GetServerState()
	if totalState != 2 && totalState != 20 && totalState != 200 {
		t.Fatal(totalState)
	}
}

func testSetupClusterWithLeader(
	t *testing.T,
) (
	*inMemoryRpcServiceHub,
	IConsensusModule, *log.InMemoryLog, *testhelpers.DummyStateMachine,
	IConsensusModule, *log.InMemoryLog, *testhelpers.DummyStateMachine,
	IConsensusModule, *log.InMemoryLog, *testhelpers.DummyStateMachine,
) {
	imrsh := &inMemoryRpcServiceHub{nil}
	setupCMR3 := func(
		thisServerId ServerId, electionTimeoutLow time.Duration,
	) (IConsensusModule, *log.InMemoryLog, *testhelpers.DummyStateMachine) {
		return setupConsensusModuleR3(
			t,
			thisServerId,
			electionTimeoutLow,
			nil,
			imrsh.getRpcService(thisServerId),
		)
	}
	cm1, diml1, dsm1 := setupCMR3(101, testdata.ElectionTimeoutLow)
	cm2, diml2, dsm2 := setupCMR3(102, testdata.ElectionTimeoutLow*3)
	cm3, diml3, dsm3 := setupCMR3(103, testdata.ElectionTimeoutLow*3)
	imrsh.cms = map[ServerId]IConsensusModule{
		101: cm1,
		102: cm2,
		103: cm3,
	}

	// -- Election timeout results in cm1 leader being elected
	time.Sleep(testdata.ElectionTimeoutLow*2 + testdata.SleepJustMoreThanATick)

	if cm1.GetServerState() != 2 || cm2.GetServerState() != 0 || cm3.GetServerState() != 0 {
		defer cm1.Stop()
		defer cm2.Stop()
		defer cm3.Stop()
		t.Fatal(cm1.GetServerState()*100 + cm2.GetServerState()*10 + cm3.GetServerState())
	}

	return imrsh, cm1, diml1, dsm1, cm2, diml2, dsm2, cm3, diml3, dsm3
}

func testSetup_SOLO_Leader(
	t *testing.T,
) (IConsensusModule, *log.InMemoryLog, *testhelpers.DummyStateMachine) {
	imrsh := &inMemoryRpcServiceHub{nil}
	cm, diml, dsm := setupConsensusModuleR3_SOLO(
		t,
		testdata.ElectionTimeoutLow,
		nil,
		imrsh.getRpcService(101),
	)
	imrsh.cms = map[ServerId]IConsensusModule{
		101: cm,
	}

	// -- Election timeout results in cm electing itself leader
	time.Sleep(testdata.ElectionTimeoutLow*2 + testdata.SleepJustMoreThanATick)

	if cm.GetServerState() != LEADER {
		defer cm.Stop()
		t.Fatal()
	}

	return cm, diml, dsm
}

func TestCluster_CommandIsReplicatedVsMissingNode(t *testing.T) {
	imrsh, cm1, diml1, dsm1, cm2, diml2, dsm2, cm3, _, _ := testSetupClusterWithLeader(t)
	defer cm1.Stop()
	defer cm2.Stop()

	// Simulate a follower crash
	imrsh.cms[103] = nil
	cm3.Stop()
	cm3 = nil

	// Apply a command on the leader
	ioleAC, result := cm1.AppendCommand(testhelpers.DummyCommand(101))

	if result != nil {
		t.Fatal()
	}
	if ioleAC != 1 {
		t.Fatal()
	}
	if iole, err := diml1.GetIndexOfLastEntry(); err != nil || iole != 1 {
		t.Fatal()
	}

	expectedLe := LogEntry{1, Command("c101")}

	// Command is in the leader's log
	le := testhelpers.TestHelper_GetLogEntryAtIndex(diml1, 1)
	if !reflect.DeepEqual(le, expectedLe) {
		t.Fatal(le)
	}
	// but not yet in connected follower's
	iole, err := diml2.GetIndexOfLastEntry()
	if err != nil || iole != 0 {
		t.Fatal()
	}

	// A tick allows command to be replicated to connected followers
	time.Sleep(testdata.TickerDuration)

	iole, err = diml2.GetIndexOfLastEntry()
	if err != nil || iole != 1 {
		t.Fatal(iole)
	}
	le = testhelpers.TestHelper_GetLogEntryAtIndex(diml2, 1)
	if !reflect.DeepEqual(le, expectedLe) {
		t.Fatal(le)
	}

	// and committed on the leader
	if dsm1.GetLastApplied() != 1 {
		t.Fatal()
	}
	if !dsm1.AppliedCommandsEqual(101) {
		t.Fatal()
	}

	// but not yet on the connected followers
	if dsm2.GetLastApplied() != 0 {
		t.Fatal()
	}

	// Another tick propagates the commit to the connected followers
	time.Sleep(testdata.TickerDuration)
	if dsm2.GetLastApplied() != 1 {
		t.Fatal()
	}
	if !dsm2.AppliedCommandsEqual(101) {
		t.Fatal()
	}

	// Crashed follower restarts
	cm3b, diml3b, dsm3b := setupConsensusModuleR3(
		t,
		103,
		testdata.ElectionTimeoutLow,
		nil,
		imrsh.getRpcService(103),
	)
	defer cm3b.Stop()
	imrsh.cms[103] = cm3b
	if dsm3b.GetLastApplied() != 0 {
		t.Fatal()
	}

	// A tick propagates the command and the commit to the recovered follower
	time.Sleep(testdata.TickerDuration)
	// FIXME: err if cm3b.GetLeader() != 101
	le = testhelpers.TestHelper_GetLogEntryAtIndex(diml3b, 1)
	if !reflect.DeepEqual(le, expectedLe) {
		t.Fatal(le)
	}
	if dsm3b.GetLastApplied() != 1 {
		t.Fatal()
	}
	if !dsm3b.AppliedCommandsEqual(101) {
		t.Fatal()
	}
}

func TestCluster_SOLO_Command_And_CommitIndexAdvance(t *testing.T) {
	cm, diml, dsm := testSetup_SOLO_Leader(t)
	defer cm.Stop()

	// Apply a command on the leader
	ioleAC, result := cm.AppendCommand(testhelpers.DummyCommand(101))

	// FIXME: sleep just enough!
	time.Sleep(testdata.SleepToLetGoroutineRun)

	if result != nil {
		t.Fatal()
	}
	if ioleAC != 1 {
		t.Fatal()
	}
	if iole, err := diml.GetIndexOfLastEntry(); err != nil || iole != 1 {
		t.Fatal()
	}

	expectedLe := LogEntry{1, Command("c101")}

	// Command is in the leader's log
	le := testhelpers.TestHelper_GetLogEntryAtIndex(diml, 1)
	if !reflect.DeepEqual(le, expectedLe) {
		t.Fatal(le)
	}
	// but not yet committed
	if dsm.GetLastApplied() != 0 {
		t.Fatal()
	}

	// A tick allows command to be committed
	time.Sleep(testdata.TickerDuration)
	if dsm.GetLastApplied() != 1 {
		t.Fatal()
	}
	if !dsm.AppliedCommandsEqual(101) {
		t.Fatal()
	}
}

// Real in-memory implementation of RpcService
// - meant only for tests
type inMemoryRpcServiceHub struct {
	cms map[ServerId]IConsensusModule
}

type inMemoryRpcServiceConnector struct {
	hub  *inMemoryRpcServiceHub
	from ServerId
}

func (imrsh *inMemoryRpcServiceHub) getRpcService(
	from ServerId,
) *inMemoryRpcServiceConnector {
	return &inMemoryRpcServiceConnector{imrsh, from}
}

func (imrs *inMemoryRpcServiceConnector) RpcAppendEntries(
	toServer ServerId,
	rpc *RpcAppendEntries,
) *RpcAppendEntriesReply {
	cm := imrs.hub.cms[toServer]
	if cm != nil {
		return cm.ProcessRpcAppendEntries(imrs.from, rpc)
	}
	return nil
}

func (imrs *inMemoryRpcServiceConnector) RpcRequestVote(
	toServer ServerId,
	rpc *RpcRequestVote,
) *RpcRequestVoteReply {
	cm := imrs.hub.cms[toServer]
	if cm != nil {
		return cm.ProcessRpcRequestVote(imrs.from, rpc)
	}
	return nil
}
