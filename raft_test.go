package raft

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func setupConsensusModule(t *testing.T, logTerms []TermNo) *ConsensusModule {
	cm, _ := setupConsensusModuleR2(t, logTerms)
	return cm
}

func setupConsensusModuleR2(
	t *testing.T,
	logTerms []TermNo,
) (*ConsensusModule, *mockRpcSender) {
	ps := newIMPSWithCurrentTerm(testCurrentTerm)
	imle := newIMLEWithDummyCommands(logTerms, testMaxEntriesPerAppendEntry)
	mrs := newMockRpcSender()
	ts := TimeSettings{testTickerDuration, testElectionTimeoutLow}
	ci, err := NewClusterInfo(testAllServerIds, testThisServerId)
	if err != nil {
		t.Fatal(err)
	}
	cm, err := NewConsensusModule(ps, imle, mrs, ci, ts)
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

	time.Sleep(testSleepJustMoreThanATick)

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

func TestConsensusModule_ProcessRpcAppendEntriesAsync(t *testing.T) {
	cm := setupConsensusModule(t, nil)
	defer cm.StopAsync()
	var serverTerm TermNo = testCurrentTerm

	replyChan := cm.ProcessRpcAppendEntriesAsync("s2", makeAEWithTerm(serverTerm-1))
	time.Sleep(testSleepToLetGoroutineRun)

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

func TestConsensusModule_ProcessRpcRequestVoteAsync(t *testing.T) {
	cm := setupConsensusModule(t, nil)
	defer cm.StopAsync()

	replyChan := cm.ProcessRpcRequestVoteAsync("s2", &RpcRequestVote{testCurrentTerm - 1, 0, 0})
	time.Sleep(testSleepToLetGoroutineRun)

	select {
	case reply := <-replyChan:
		serverTerm := cm.passiveConsensusModule.persistentState.GetCurrentTerm()
		expectedRpc := RpcRequestVoteReply{serverTerm, false}
		if *reply != expectedRpc {
			t.Fatal(reply)
		}
	default:
		t.Fatal()
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
	mrs *mockRpcSender,
) {
	// FIXME: multiple unsafe concurrent accesses

	ett := cm.passiveConsensusModule.electionTimeoutTracker
	time.Sleep(ett.currentElectionTimeout + testSleepJustMoreThanATick)

	if cm.GetServerState() != CANDIDATE {
		t.Fatal()
	}

	// candidate has issued RequestVote RPCs to all other servers.
	lastLogIndex, lastLogTerm, err := GetIndexAndTermOfLastEntry(cm.passiveConsensusModule.lasm)
	if err != nil {
		t.Fatal(err)
	}
	expectedRpc := &RpcRequestVote{testCurrentTerm + 1, lastLogIndex, lastLogTerm}
	expectedRpcs := []mockSentRpc{
		{"s2", expectedRpc},
		{"s3", expectedRpc},
		{"s4", expectedRpc},
		{"s5", expectedRpc},
	}
	mrs.checkSentRpcs(t, expectedRpcs)

	// reply true for all votes
	serverTerm := cm.passiveConsensusModule.persistentState.GetCurrentTerm()
	if mrs.sendReplies(&RpcRequestVoteReply{serverTerm, true}) != 4 {
		t.Fatal()
	}

	time.Sleep(testSleepToLetGoroutineRun)

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
		cm.passiveConsensusModule.getCommitIndex(),
	}
	expectedRpcs2 := []mockSentRpc{
		{"s2", expectedRpc2},
		{"s3", expectedRpc2},
		{"s4", expectedRpc2},
		{"s5", expectedRpc2},
	}
	mrs.checkSentRpcs(t, expectedRpcs2)

	// reply handling
	expectedMatchIndex := map[ServerId]LogIndex{"s2": 0, "s3": 0, "s4": 0, "s5": 0}
	if !reflect.DeepEqual(cm.passiveConsensusModule.leaderVolatileState.matchIndex, expectedMatchIndex) {
		t.Fatal()
	}

	if mrs.sendReplies(&RpcAppendEntriesReply{serverTerm, true}) != 4 {
		t.Fatal()
	}
	time.Sleep(testSleepToLetGoroutineRun)

	expectedMatchIndex = map[ServerId]LogIndex{
		"s2": lastLogIndex,
		"s3": lastLogIndex,
		"s4": lastLogIndex,
		"s5": lastLogIndex,
	}
	if !reflect.DeepEqual(cm.passiveConsensusModule.leaderVolatileState.matchIndex, expectedMatchIndex) {
		t.Fatal(cm.passiveConsensusModule.leaderVolatileState.matchIndex)
	}
}

func TestConsensusModule_AppendCommandAsync_Leader(t *testing.T) {
	cm, mrs := setupConsensusModuleR2(t, makeLogTerms_Figure7LeaderLine())
	defer cm.StopAsync()

	testConsensusModule_RpcReplyCallback_AndBecomeLeader(t, cm, mrs)

	// pre check
	iole, err := cm.passiveConsensusModule.lasm.GetIndexOfLastEntry()
	if err != nil {
		t.Fatal()
	}
	if iole != 10 {
		t.Fatal()
	}

	command := Command("c11x")
	replyChan := cm.AppendCommandAsync(command)

	iole, err = cm.passiveConsensusModule.lasm.GetIndexOfLastEntry()
	if err != nil {
		t.Fatal()
	}
	if iole != 10 {
		t.Fatal()
	}

	time.Sleep(testSleepToLetGoroutineRun)

	select {
	case reply := <-replyChan:
		if cm.IsStopped() {
			t.Error(cm.GetStopError())
		}
		expectedReply := AppendCommandResult{11, nil}
		if !reflect.DeepEqual(reply, expectedReply) {
			t.Fatal(reply)
		}
		iole, err = cm.passiveConsensusModule.lasm.GetIndexOfLastEntry()
		if err != nil {
			t.Fatal()
		}
		if iole != 11 {
			t.Fatal()
		}
		le := testHelper_GetLogEntryAtIndex(cm.passiveConsensusModule.lasm, 11)
		if !reflect.DeepEqual(le, LogEntry{8, Command("c11x")}) {
			t.Fatal(le)
		}
	default:
		t.Fatal()
	}
}

func TestConsensusModule_AppendCommandAsync_Follower(t *testing.T) {
	cm, _ := setupConsensusModuleR2(t, makeLogTerms_Figure7LeaderLine())
	defer cm.StopAsync()

	// pre check
	iole, err := cm.passiveConsensusModule.lasm.GetIndexOfLastEntry()
	if err != nil {
		t.Fatal()
	}
	if iole != 10 {
		t.Fatal()
	}

	command := Command("c11x")
	replyChan := cm.AppendCommandAsync(command)

	iole, err = cm.passiveConsensusModule.lasm.GetIndexOfLastEntry()
	if err != nil {
		t.Fatal()
	}
	if iole != 10 {
		t.Fatal()
	}

	time.Sleep(testSleepToLetGoroutineRun)

	select {
	case reply := <-replyChan:
		if cm.IsStopped() {
			t.Error(cm.GetStopError())
		}
		expectedReply := AppendCommandResult{
			0,
			errors.New("raft: state != LEADER - cannot append command to log"),
		}
		if !reflect.DeepEqual(reply, expectedReply) {
			t.Fatal(reply)
		}
		iole, err := cm.passiveConsensusModule.lasm.GetIndexOfLastEntry()
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
