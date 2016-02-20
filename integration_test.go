package raft

import (
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
) (*ConsensusModule, *inMemoryLog) {
	ps := newIMPSWithCurrentTerm(0)
	imle := newIMLEWithDummyCommands(logTerms, testMaxEntriesPerAppendEntry)
	ts := TimeSettings{testTickerDuration, electionTimeoutLow}
	ci := NewClusterInfo(testClusterServerIds, thisServerId)
	cm := NewConsensusModule(ps, imle, imrsc, ci, ts)
	if cm == nil {
		t.Fatal()
	}
	return cm, imle
}

func TestCluster_ElectsLeader(t *testing.T) {
	imrsh := &inMemoryRpcServiceHub{nil}
	setupCMR3 := func(thisServerId ServerId) *ConsensusModule {
		cm, _ := setupConsensusModuleR3(
			t,
			thisServerId,
			testElectionTimeoutLow,
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

	time.Sleep(testElectionTimeoutLow - testSleepToLetGoroutineRun)

	totalState = cm1.GetServerState() + cm2.GetServerState() + cm3.GetServerState()
	if totalState != 0 {
		t.Fatal(totalState)
	}

	// -- Election timeout results in a leader being elected
	// (note: this test has a low probability race condition where two nodes
	// can become candidates at the same time and no leader is elected)
	time.Sleep(testSleepToLetGoroutineRun + testElectionTimeoutLow + testSleepJustMoreThanATick)

	totalState = cm1.GetServerState() + cm2.GetServerState() + cm3.GetServerState()
	if totalState != 2 {
		t.Fatal(totalState)
	}
	if cm1.GetServerState() == 1 || cm2.GetServerState() == 1 || cm3.GetServerState() == 1 {
		t.Fatal(totalState)
	}
}

func testSetupClusterWithLeader(
	t *testing.T,
) (
	*inMemoryRpcServiceHub,
	*ConsensusModule, *inMemoryLog,
	*ConsensusModule, *inMemoryLog,
	*ConsensusModule, *inMemoryLog,
) {
	imrsh := &inMemoryRpcServiceHub{nil}
	setupCMR3 := func(
		thisServerId ServerId, electionTimeoutLow time.Duration,
	) (*ConsensusModule, *inMemoryLog) {
		return setupConsensusModuleR3(
			t,
			thisServerId,
			electionTimeoutLow,
			nil,
			imrsh.getRpcService(thisServerId),
		)
	}
	cm1, imle1 := setupCMR3("s1", testElectionTimeoutLow/2)
	cm2, imle2 := setupCMR3("s2", testElectionTimeoutLow)
	cm3, imle3 := setupCMR3("s3", testElectionTimeoutLow)
	imrsh.cms = map[ServerId]*ConsensusModule{
		"s1": cm1,
		"s2": cm2,
		"s3": cm3,
	}

	// -- Election timeout results in cm1 leader being elected
	time.Sleep(testElectionTimeoutLow + testSleepJustMoreThanATick)

	if cm1.GetServerState() != 2 || cm2.GetServerState() != 0 || cm3.GetServerState() != 0 {
		defer cm1.StopAsync()
		defer cm2.StopAsync()
		defer cm3.StopAsync()
		t.Fatal()
	}

	return imrsh, cm1, imle1, cm2, imle2, cm3, imle3
}

func TestCluster_CommandIsReplicatedVsMissingNode(t *testing.T) {
	imrsh, cm1, imle1, cm2, imle2, cm3, _ := testSetupClusterWithLeader(t)
	defer cm1.StopAsync()
	defer cm2.StopAsync()

	// Simulate a follower crash
	imrsh.cms["s3"] = nil
	cm3.StopAsync()
	cm3 = nil

	// Apply a command on the leader
	command := Command("c101")
	replyChan := cm1.AppendCommandAsync(command)

	// FIXME: sleep just enough!
	time.Sleep(testSleepToLetGoroutineRun)
	select {
	case reply := <-replyChan:
		if !reflect.DeepEqual(reply, AppendCommandResult{1, nil}) {
			t.Fatal(reply)
		}
	default:
		t.Fatal()
	}

	expectedLe := LogEntry{1, Command("c101")}

	// Command is in the leader's log
	le := testHelper_GetLogEntryAtIndex(imle1, 1)
	if !reflect.DeepEqual(le, expectedLe) {
		t.Fatal(le)
	}
	// but not yet in connected follower's
	iole, err := imle2.GetIndexOfLastEntry()
	if err != nil || iole != 0 {
		t.Fatal()
	}

	// A tick allows command to be replicated to connected followers
	time.Sleep(testTickerDuration)

	iole, err = imle2.GetIndexOfLastEntry()
	if err != nil || iole != 1 {
		t.Fatal()
	}
	le = testHelper_GetLogEntryAtIndex(imle2, 1)
	if !reflect.DeepEqual(le, expectedLe) {
		t.Fatal(le)
	}

	// and committed on the leader
	if imle1._commitIndex != 1 {
		t.Fatal()
	}
	// but not yet on the connected followers
	if imle2._commitIndex != 0 {
		t.Fatal()
	}

	// Another tick propagates the commit to the connected followers
	time.Sleep(testTickerDuration)
	if imle2._commitIndex != 1 {
		t.Fatal()
	}

	// Crashed follower restarts
	cm3b, imle3b := setupConsensusModuleR3(
		t,
		"s3",
		testElectionTimeoutLow,
		nil,
		imrsh.getRpcService("s3"),
	)
	defer cm3b.StopAsync()
	imrsh.cms["s3"] = cm3b
	if imle3b._commitIndex != 0 {
		t.Fatal()
	}

	// A tick propagates the command and the commit to the recovered follower
	time.Sleep(testTickerDuration)
	// FIXME: err if cm3b.GetLeader() != "s1"
	le = testHelper_GetLogEntryAtIndex(imle3b, 1)
	if !reflect.DeepEqual(le, expectedLe) {
		t.Fatal(le)
	}
	if imle3b._commitIndex != 1 {
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

func (imrsh *inMemoryRpcServiceHub) getRpcService(from ServerId) *inMemoryRpcServiceConnector {
	return &inMemoryRpcServiceConnector{imrsh, from}
}

func (imrs *inMemoryRpcServiceConnector) SendRpcAppendEntriesAsync(
	toServer ServerId,
	rpc *RpcAppendEntries,
	replyAsync func(*RpcAppendEntriesReply),
) {
	cm := imrs.hub.cms[toServer]
	if cm != nil {
		replyChan := cm.ProcessRpcAppendEntriesAsync(imrs.from, rpc)
		go func() {
			replyAsync(<-replyChan)
		}()
	}
}

func (imrs *inMemoryRpcServiceConnector) SendRpcRequestVoteAsync(
	toServer ServerId,
	rpc *RpcRequestVote,
	replyAsync func(*RpcRequestVoteReply),
) {
	cm := imrs.hub.cms[toServer]
	if cm != nil {
		replyChan := cm.ProcessRpcRequestVoteAsync(imrs.from, rpc)
		go func() {
			replyAsync(<-replyChan)
		}()
	}
}
