package impl

import (
	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/config"
	"github.com/divtxt/raft/lasm"
	"github.com/divtxt/raft/log"
	"github.com/divtxt/raft/rps"
	"github.com/divtxt/raft/testdata"
	"github.com/divtxt/raft/testhelpers"
	"reflect"
	"testing"
	"time"
)

var testClusterServerIds = []ServerId{"s1", "s2", "s3"}

func setupConsensusModuleR3(
	t *testing.T,
	thisServerId ServerId,
	electionTimeoutLow time.Duration,
	logTerms []TermNo,
	imrsc *inMemoryRpcServiceConnector,
) (*ConsensusModule, *lasm.LogAndStateMachineImpl) {
	ps := rps.NewIMPSWithCurrentTerm(0)
	diml := lasm.TestUtil_NewLasmiWithDummyCommands(logTerms, testdata.MaxEntriesPerAppendEntry)
	ts := config.TimeSettings{testdata.TickerDuration, electionTimeoutLow}
	ci, err := config.NewClusterInfo(testClusterServerIds, thisServerId)
	if err != nil {
		t.Fatal(err)
	}
	cm, err := NewConsensusModule(ps, diml, imrsc, ci, ts)
	if err != nil {
		t.Fatal(err)
	}
	if cm == nil {
		t.Fatal()
	}
	return cm, diml
}

func setupConsensusModuleR3_SOLO(
	t *testing.T,
	electionTimeoutLow time.Duration,
	logTerms []TermNo,
	imrsc *inMemoryRpcServiceConnector,
) (*ConsensusModule, *lasm.LogAndStateMachineImpl) {
	ps := rps.NewIMPSWithCurrentTerm(0)
	diml := lasm.TestUtil_NewLasmiWithDummyCommands(logTerms, testdata.MaxEntriesPerAppendEntry)
	ts := config.TimeSettings{testdata.TickerDuration, testdata.ElectionTimeoutLow}
	ci, err := config.NewClusterInfo([]ServerId{"_SOLO_"}, "_SOLO_")
	if err != nil {
		t.Fatal(err)
	}
	cm, err := NewConsensusModule(ps, diml, imrsc, ci, ts)
	if err != nil {
		t.Fatal(err)
	}
	if cm == nil {
		t.Fatal()
	}
	return cm, diml
}

func TestCluster_ElectsLeader(t *testing.T) {
	imrsh := &inMemoryRpcServiceHub{nil}
	setupCMR3 := func(thisServerId ServerId) *ConsensusModule {
		cm, _ := setupConsensusModuleR3(
			t,
			thisServerId,
			testdata.ElectionTimeoutLow,
			nil,
			imrsh.getRpcService(thisServerId),
		)
		return cm
	}
	cm1 := setupCMR3("s1")
	defer cm1.StopAsync()
	cm2 := setupCMR3("s2")
	defer cm2.StopAsync()
	cm3 := setupCMR3("s3")
	defer cm3.StopAsync()
	imrsh.cms = map[ServerId]*ConsensusModule{
		"s1": cm1,
		"s2": cm2,
		"s3": cm3,
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
	*ConsensusModule, *lasm.LogAndStateMachineImpl,
	*ConsensusModule, *lasm.LogAndStateMachineImpl,
	*ConsensusModule, *lasm.LogAndStateMachineImpl,
) {
	imrsh := &inMemoryRpcServiceHub{nil}
	setupCMR3 := func(
		thisServerId ServerId, electionTimeoutLow time.Duration,
	) (*ConsensusModule, *lasm.LogAndStateMachineImpl) {
		return setupConsensusModuleR3(
			t,
			thisServerId,
			electionTimeoutLow,
			nil,
			imrsh.getRpcService(thisServerId),
		)
	}
	cm1, diml1 := setupCMR3("s1", testdata.ElectionTimeoutLow)
	cm2, diml2 := setupCMR3("s2", testdata.ElectionTimeoutLow*3)
	cm3, diml3 := setupCMR3("s3", testdata.ElectionTimeoutLow*3)
	imrsh.cms = map[ServerId]*ConsensusModule{
		"s1": cm1,
		"s2": cm2,
		"s3": cm3,
	}

	// -- Election timeout results in cm1 leader being elected
	time.Sleep(testdata.ElectionTimeoutLow*2 + testdata.SleepJustMoreThanATick)

	if cm1.GetServerState() != 2 || cm2.GetServerState() != 0 || cm3.GetServerState() != 0 {
		defer cm1.StopAsync()
		defer cm2.StopAsync()
		defer cm3.StopAsync()
		t.Fatal(cm1.GetServerState()*100 + cm2.GetServerState()*10 + cm3.GetServerState())
	}

	return imrsh, cm1, diml1, cm2, diml2, cm3, diml3
}

func testSetup_SOLO_Leader(t *testing.T) (*ConsensusModule, *lasm.LogAndStateMachineImpl) {
	imrsh := &inMemoryRpcServiceHub{nil}
	cm, diml := setupConsensusModuleR3_SOLO(
		t,
		testdata.ElectionTimeoutLow,
		nil,
		imrsh.getRpcService("_SOLO_"),
	)
	imrsh.cms = map[ServerId]*ConsensusModule{
		"_SOLO_": cm,
	}

	// -- Election timeout results in cm electing itself leader
	time.Sleep(testdata.ElectionTimeoutLow*2 + testdata.SleepJustMoreThanATick)

	if cm.GetServerState() != LEADER {
		defer cm.StopAsync()
		t.Fatal()
	}

	return cm, diml
}

func TestCluster_CommandIsReplicatedVsMissingNode(t *testing.T) {
	imrsh, cm1, diml1, cm2, diml2, cm3, _ := testSetupClusterWithLeader(t)
	defer cm1.StopAsync()
	defer cm2.StopAsync()

	// Simulate a follower crash
	imrsh.cms["s3"] = nil
	cm3.StopAsync()
	cm3 = nil

	// Apply a command on the leader
	replyChan := cm1.AppendCommandAsync(testhelpers.DummyCommand{101, false})

	// FIXME: sleep just enough!
	time.Sleep(testdata.SleepToLetGoroutineRun)
	select {
	case result := <-replyChan:
		if result != testhelpers.DummyCommand_Reply_Ok {
			t.Fatal()
		}
		if iole, err := diml1.GetIndexOfLastEntry(); err != nil || iole != 1 {
			t.Fatal()
		}
	default:
		t.Fatal()
	}

	expectedLe := LogEntry{1, Command("c101")}

	// Command is in the leader's log
	le := log.TestHelper_GetLogEntryAtIndex(diml1, 1)
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
		t.Fatal()
	}
	le = log.TestHelper_GetLogEntryAtIndex(diml2, 1)
	if !reflect.DeepEqual(le, expectedLe) {
		t.Fatal(le)
	}

	// and committed on the leader
	if diml1.GetCommitIndex() != 1 {
		t.Fatal()
	}
	// but not yet on the connected followers
	if diml2.GetCommitIndex() != 0 {
		t.Fatal()
	}

	// Another tick propagates the commit to the connected followers
	time.Sleep(testdata.TickerDuration)
	if diml2.GetCommitIndex() != 1 {
		t.Fatal()
	}

	// Crashed follower restarts
	cm3b, diml3b := setupConsensusModuleR3(
		t,
		"s3",
		testdata.ElectionTimeoutLow,
		nil,
		imrsh.getRpcService("s3"),
	)
	defer cm3b.StopAsync()
	imrsh.cms["s3"] = cm3b
	if diml3b.GetCommitIndex() != 0 {
		t.Fatal()
	}

	// A tick propagates the command and the commit to the recovered follower
	time.Sleep(testdata.TickerDuration)
	// FIXME: err if cm3b.GetLeader() != "s1"
	le = log.TestHelper_GetLogEntryAtIndex(diml3b, 1)
	if !reflect.DeepEqual(le, expectedLe) {
		t.Fatal(le)
	}
	if diml3b.GetCommitIndex() != 1 {
		t.Fatal()
	}
}

func TestCluster_SOLO_Command_And_CommitIndexAdvance(t *testing.T) {
	cm, diml := testSetup_SOLO_Leader(t)
	defer cm.StopAsync()

	// Apply a command on the leader
	replyChan := cm.AppendCommandAsync(testhelpers.DummyCommand{101, false})

	// FIXME: sleep just enough!
	time.Sleep(testdata.SleepToLetGoroutineRun)
	select {
	case result := <-replyChan:
		if result != testhelpers.DummyCommand_Reply_Ok {
			t.Fatal()
		}
		if iole, err := diml.GetIndexOfLastEntry(); err != nil || iole != 1 {
			t.Fatal()
		}
	default:
		t.Fatal()
	}

	expectedLe := LogEntry{1, Command("c101")}

	// Command is in the leader's log
	le := log.TestHelper_GetLogEntryAtIndex(diml, 1)
	if !reflect.DeepEqual(le, expectedLe) {
		t.Fatal(le)
	}
	// but not yet committed
	if diml.GetCommitIndex() != 0 {
		t.Fatal()
	}

	// A tick allows command to be committed
	time.Sleep(testdata.TickerDuration)
	if diml.GetCommitIndex() != 1 {
		t.Fatal()
	}
}

// Real in-memory implementation of RpcService
// - meant only for tests
type inMemoryRpcServiceHub struct {
	cms map[ServerId]*ConsensusModule
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

func (imrs *inMemoryRpcServiceConnector) SendRpcAppendEntriesAsync(
	toServer ServerId,
	rpc *RpcAppendEntries,
	replyAsync func(*RpcAppendEntriesReply),
) error {
	cm := imrs.hub.cms[toServer]
	if cm != nil {
		replyChan := cm.ProcessRpcAppendEntriesAsync(imrs.from, rpc)
		go func() {
			replyAsync(<-replyChan)
		}()
	}
	return nil
}

func (imrs *inMemoryRpcServiceConnector) SendRpcRequestVoteAsync(
	toServer ServerId,
	rpc *RpcRequestVote,
	replyAsync func(*RpcRequestVoteReply),
) error {
	cm := imrs.hub.cms[toServer]
	if cm != nil {
		replyChan := cm.ProcessRpcRequestVoteAsync(imrs.from, rpc)
		go func() {
			replyAsync(<-replyChan)
		}()
	}
	return nil
}
