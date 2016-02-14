package raft

import (
	"testing"
	"time"
)

var testClusterServerIds = []ServerId{"s1", "s2", "s3"}

func setupConsensusModuleR3(
	t *testing.T,
	thisServerId ServerId,
	logTerms []TermNo,
	imrsc *inMemoryRpcServiceConnector,
) *ConsensusModule {
	ps := newIMPSWithCurrentTerm(testCurrentTerm)
	imle := newIMLEWithDummyCommands(logTerms, testMaxEntriesPerAppendEntry)
	ts := TimeSettings{testTickerDuration, testElectionTimeoutLow}
	ci := NewClusterInfo(testClusterServerIds, thisServerId)
	cm := NewConsensusModule(ps, imle, imrsc, ci, ts)
	if cm == nil {
		t.Fatal()
	}
	return cm
}

func TestNewClusterElectsLeader(t *testing.T) {
	imrsh := &inMemoryRpcServiceHub{nil}
	setupCMR3 := func(thisServerId ServerId) *ConsensusModule {
		return setupConsensusModuleR3(t, thisServerId, nil, imrsh.getRpcService(thisServerId))
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

	totalState := cm1.GetServerState() + cm2.GetServerState() + cm3.GetServerState()
	if totalState != 0 {
		t.Fatal(totalState)
	}

	time.Sleep(testElectionTimeoutLow - testSleepToLetGoroutineRun)

	totalState = cm1.GetServerState() + cm2.GetServerState() + cm3.GetServerState()
	if totalState != 0 {
		t.Fatal(totalState)
	}

	time.Sleep(testElectionTimeoutLow + testSleepJustMoreThanATick)

	totalState = cm1.GetServerState() + cm2.GetServerState() + cm3.GetServerState()
	if totalState != 2 {
		t.Fatal(totalState)
	}
	if cm1.GetServerState() == 1 || cm2.GetServerState() == 1 || cm3.GetServerState() == 1 {
		t.Fatal(totalState)
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
	replyChan := cm.ProcessRpcAppendEntriesAsync(imrs.from, rpc)
	go func() {
		replyAsync(<-replyChan)
	}()
}

func (imrs *inMemoryRpcServiceConnector) SendRpcRequestVoteAsync(
	toServer ServerId,
	rpc *RpcRequestVote,
	replyAsync func(*RpcRequestVoteReply),
) {
	cm := imrs.hub.cms[toServer]
	replyChan := cm.ProcessRpcRequestVoteAsync(imrs.from, rpc)
	go func() {
		replyAsync(<-replyChan)
	}()
}
