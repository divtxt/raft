package raft

import (
	"errors"
	"fmt"
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
	ci := NewClusterInfo(testAllServerIds, testThisServerId)
	cm := NewConsensusModule(ps, imle, mrs, ci, ts)
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
	// FIXME: unsafe concurrent access
	ett := cm.passiveConsensusModule.electionTimeoutTracker
	time.Sleep(ett.currentElectionTimeout + testSleepJustMoreThanATick)

	if cm.GetServerState() != CANDIDATE {
		t.Fatal()
	}

	// candidate has issued RequestVote RPCs to all other servers.
	lastLogIndex, lastLogTerm := GetIndexAndTermOfLastEntry(cm.passiveConsensusModule.log)
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
}

func TestConsensusModule_AppendCommandAsync_Leader(t *testing.T) {
	cm, mrs := setupConsensusModuleR2(t, makeLogTerms_Figure7LeaderLine())
	defer cm.StopAsync()

	testConsensusModule_RpcReplyCallback_AndBecomeLeader(t, cm, mrs)

	// pre check
	if cm.passiveConsensusModule.log.GetIndexOfLastEntry() != 10 {
		t.Fatal()
	}

	command := Command("c11x")
	replyChan := cm.AppendCommandAsync(command)

	if cm.passiveConsensusModule.log.GetIndexOfLastEntry() != 10 {
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
		if cm.passiveConsensusModule.log.GetIndexOfLastEntry() != 11 {
			t.Fatal()
		}
		le := cm.passiveConsensusModule.log.GetLogEntryAtIndex(11)
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
	if cm.passiveConsensusModule.log.GetIndexOfLastEntry() != 10 {
		t.Fatal()
	}

	command := Command("c11x")
	replyChan := cm.AppendCommandAsync(command)

	if cm.passiveConsensusModule.log.GetIndexOfLastEntry() != 10 {
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
		if cm.passiveConsensusModule.log.GetIndexOfLastEntry() != 10 {
			t.Fatal()
		}
	default:
		t.Fatal()
	}
}
