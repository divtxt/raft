package raft

import (
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
	imle := newIMLEWithDummyCommands(logTerms)
	mrs := newMockRpcSender()
	ts := TimeSettings{testTickerDuration, testElectionTimeoutLow}
	cm := NewConsensusModule(ps, imle, mrs, testServerId, testPeerIds, ts)
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

func TestConsensusModule_UnknownRpcTypeStopsCM(t *testing.T) {
	cm := setupConsensusModule(t, nil)

	if cm.IsStopped() {
		t.Error()
	}

	cm.ProcessRpcAsync("s2", &struct{ int }{42})
	time.Sleep(testSleepToLetGoroutineRun)

	if !cm.IsStopped() {
		cm.StopAsync()
		t.Fatal()
	}

	e := cm.GetStopError()
	if e != "FATAL: unknown rpc type: *struct { int } from: s2" {
		t.Error(e)
	}
}

func TestConsensusModule_ProcessRpcAsync(t *testing.T) {
	cm := setupConsensusModule(t, nil)
	defer cm.StopAsync()

	replyChan := cm.ProcessRpcAsync("s2", &RpcRequestVote{testCurrentTerm - 1, 0, 0})
	time.Sleep(testSleepToLetGoroutineRun)

	select {
	case reply := <-replyChan:
		// FIXME: unsafe concurrent access
		serverTerm := cm.passiveConsensusModule.persistentState.GetCurrentTerm()
		expectedRpc := &RpcRequestVoteReply{serverTerm, false}
		if !reflect.DeepEqual(reply, expectedRpc) {
			t.Fatal(reply)
		}
	default:
		t.Fatal()
	}
}
