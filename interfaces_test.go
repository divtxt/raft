package raft

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"testing"
)

// -- RaftPersistentState

// RaftPersistentState blackbox test.
// Send a RaftPersistentState in new / reset state.
func PartialTest_RaftPersistentState_BlackboxTest(t *testing.T, raftPersistentState RaftPersistentState) {
	// Initial data tests
	if raftPersistentState.GetCurrentTerm() != 0 {
		t.Fatal()
	}
	if raftPersistentState.GetVotedFor() != "" {
		t.Fatal()
	}

	// Set currentTerm to 0 is an error
	err := raftPersistentState.SetCurrentTerm(0)
	if err.Error() != "FATAL: attempt to set currentTerm to 0" {
		t.Fatal(err)
	}
	if raftPersistentState.GetCurrentTerm() != 0 {
		t.Fatal()
	}
	// Set votedFor while currentTerm is 0 is an error
	err = raftPersistentState.SetVotedFor("s1")
	if err.Error() != "FATAL: attempt to set votedFor while currentTerm is 0" {
		t.Fatal(err)
	}
	if raftPersistentState.GetCurrentTerm() != 0 {
		t.Fatal()
	}
	// Set currentTerm greater is ok, clears votedFor
	err = raftPersistentState.SetCurrentTerm(1)
	if err != nil {
		t.Fatal(err)
	}
	if raftPersistentState.GetCurrentTerm() != 1 {
		t.Fatal()
	}
	// Set votedFor of blank is an error
	err = raftPersistentState.SetVotedFor("")
	if err.Error() != "FATAL: attempt to set blank votedFor" {
		t.Fatal(err)
	}
	if raftPersistentState.GetVotedFor() != "" {
		t.Fatal()
	}
	// Set votedFor is ok
	err = raftPersistentState.SetVotedFor("s1")
	if err != nil {
		t.Fatal(err)
	}
	if raftPersistentState.GetVotedFor() != "s1" {
		t.Fatal()
	}
	// Set currentTerm greater is ok, clears votedFor
	err = raftPersistentState.SetCurrentTerm(4)
	if err != nil {
		t.Fatal(err)
	}
	if raftPersistentState.GetCurrentTerm() != 4 {
		t.Fatal()
	}
	if raftPersistentState.GetVotedFor() != "" {
		t.Fatal(raftPersistentState.GetVotedFor())
	}
	// Set votedFor while blank is ok
	err = raftPersistentState.SetVotedFor("s2")
	if err != nil {
		t.Fatal(err)
	}
	if raftPersistentState.GetVotedFor() != "s2" {
		t.Fatal()
	}
	// Set currentTerm same is ok, does not affect votedFor
	err = raftPersistentState.SetCurrentTerm(4)
	if err != nil {
		t.Fatal(err)
	}
	if raftPersistentState.GetCurrentTerm() != 4 {
		t.Fatal()
	}
	if raftPersistentState.GetVotedFor() != "s2" {
		t.Fatal()
	}
	// Set currentTerm less is an error
	err = raftPersistentState.SetCurrentTerm(3)
	if err.Error() != "FATAL: attempt to decrease currentTerm: 4 to 3" {
		t.Fatal(err)
	}
	if raftPersistentState.GetCurrentTerm() != 4 {
		t.Fatal()
	}
	// Set votedFor while not blank is an error
	err = raftPersistentState.SetVotedFor("s3")
	if err.Error() != "FATAL: attempt to change non-blank votedFor: s2 to s3" {
		t.Fatal(err)
	}
	if raftPersistentState.GetVotedFor() != "s2" {
		t.Fatal()
	}
}

// In-memory implementation of RaftPersistentState - meant only for tests
type inMemoryRaftPersistentState struct {
	mutex       *sync.Mutex
	currentTerm TermNo
	votedFor    ServerId
}

func (imps *inMemoryRaftPersistentState) GetCurrentTerm() TermNo {
	imps.mutex.Lock()
	defer imps.mutex.Unlock()
	return imps.currentTerm
}

func (imps *inMemoryRaftPersistentState) GetVotedFor() ServerId {
	imps.mutex.Lock()
	defer imps.mutex.Unlock()
	return imps.votedFor
}

func (imps *inMemoryRaftPersistentState) SetCurrentTerm(currentTerm TermNo) error {
	imps.mutex.Lock()
	defer imps.mutex.Unlock()
	if currentTerm == 0 {
		return errors.New("FATAL: attempt to set currentTerm to 0")
	}
	if currentTerm < imps.currentTerm {
		return fmt.Errorf("FATAL: attempt to decrease currentTerm: %v to %v", imps.currentTerm, currentTerm)
	}
	if currentTerm > imps.currentTerm {
		imps.votedFor = ""
	}
	imps.currentTerm = currentTerm
	return nil
}

func (imps *inMemoryRaftPersistentState) SetVotedFor(votedFor ServerId) error {
	imps.mutex.Lock()
	defer imps.mutex.Unlock()
	if imps.currentTerm == 0 {
		return errors.New("FATAL: attempt to set votedFor while currentTerm is 0")
	}
	if votedFor == "" {
		return errors.New("FATAL: attempt to set blank votedFor")
	}
	if imps.votedFor != "" {
		return fmt.Errorf("FATAL: attempt to change non-blank votedFor: %v to %v", imps.votedFor, votedFor)
	}
	imps.votedFor = votedFor
	return nil
}

func newIMPSWithCurrentTerm(currentTerm TermNo) *inMemoryRaftPersistentState {
	return &inMemoryRaftPersistentState{&sync.Mutex{}, currentTerm, ""}
}

// Run the blackbox test on inMemoryRaftPersistentState
func TestInMemoryRaftPersistentState(t *testing.T) {
	imps := newIMPSWithCurrentTerm(0)
	PartialTest_RaftPersistentState_BlackboxTest(t, imps)
}

// -- rpcSender

// Mock in-memory implementation of both RpcService & rpcSender
// - meant only for tests
type mockRpcSender struct {
	c           chan mockSentRpc
	replyAsyncs chan func(interface{})
}

type mockSentRpc struct {
	toServer ServerId
	rpc      interface{}
}

func newMockRpcSender() *mockRpcSender {
	return &mockRpcSender{
		make(chan mockSentRpc, 100),
		make(chan func(interface{}), 100),
	}
}

func (mrs *mockRpcSender) sendRpcAppendEntriesAsync(toServer ServerId, rpc *RpcAppendEntries) error {
	return mrs.SendRpcAppendEntriesAsync(toServer, rpc, nil)
}

func (mrs *mockRpcSender) sendRpcRequestVoteAsync(toServer ServerId, rpc *RpcRequestVote) error {
	return mrs.SendRpcRequestVoteAsync(toServer, rpc, nil)
}

func (mrs *mockRpcSender) SendRpcAppendEntriesAsync(
	toServer ServerId,
	rpc *RpcAppendEntries,
	replyAsync func(*RpcAppendEntriesReply),
) error {
	select {
	default:
		return errors.New("oops!")
	case mrs.c <- mockSentRpc{toServer, rpc}:
		if replyAsync != nil {
			mrs.replyAsyncs <- func(rpcReply interface{}) {
				switch rpcReply := rpcReply.(type) {
				case *RpcAppendEntriesReply:
					replyAsync(rpcReply)
				default:
					panic("oops!")
				}
			}
		}
	}
	return nil
}

func (mrs *mockRpcSender) SendRpcRequestVoteAsync(
	toServer ServerId,
	rpc *RpcRequestVote,
	replyAsync func(*RpcRequestVoteReply),
) error {
	select {
	default:
		return errors.New("oops!")
	case mrs.c <- mockSentRpc{toServer, rpc}:
		if replyAsync != nil {
			mrs.replyAsyncs <- func(rpcReply interface{}) {
				switch rpcReply := rpcReply.(type) {
				case *RpcRequestVoteReply:
					replyAsync(rpcReply)
				default:
					panic("oops!")
				}
			}
		}
	}
	return nil
}

// Clear sent rpcs.
func (mrs *mockRpcSender) clearSentRpcs() {
loop:
	for {
		select {
		case <-mrs.c:
		default:
			break loop
		}
	}
}

// Clears & checks sent rpcs.
// expectedRpcs should be sorted by server
func (mrs *mockRpcSender) checkSentRpcs(t *testing.T, expectedRpcs []mockSentRpc) {
	rpcs := make([]mockSentRpc, 0, 100)

loop:
	for {
		select {
		case v := <-mrs.c:
			n := len(rpcs)
			rpcs = rpcs[0 : n+1]
			rpcs[n] = v
		default:
			break loop
		}
	}

	sort.Sort(mockRpcSenderSlice(rpcs))

	if len(rpcs) != len(expectedRpcs) {
		t.Fatal(fmt.Sprintf("Expected len: %v; got len: %v", len(expectedRpcs), len(rpcs)))
	}
	diffs := false
	for i := 0; i < len(rpcs); i++ {
		if !reflect.DeepEqual(rpcs[i], expectedRpcs[i]) {
			t.Error(fmt.Sprintf(
				"diff at [%v] - expected: [{%v %v}]; got: [{%v %v}]",
				i,
				expectedRpcs[i].toServer, expectedRpcs[i].rpc,
				rpcs[i].toServer, rpcs[i].rpc,
			))
			diffs = true
		}
	}
	if diffs {
		t.Fatal(fmt.Sprintf("Expected: %v; got: %v", expectedRpcs, rpcs))
	}
}

// Clears & sends reply to sent reply functions
func (mrs *mockRpcSender) sendReplies(reply interface{}) int {
	var n int = 0
loop:
	for {
		select {
		case replyAsync := <-mrs.replyAsyncs:
			replyAsync(reply)
			n++
		default:
			break loop
		}
	}
	return n
}

// implement sort.Interface for mockSentRpc slices
type mockRpcSenderSlice []mockSentRpc

func (mrss mockRpcSenderSlice) Len() int           { return len(mrss) }
func (mrss mockRpcSenderSlice) Less(i, j int) bool { return mrss[i].toServer < mrss[j].toServer }
func (mrss mockRpcSenderSlice) Swap(i, j int)      { mrss[i], mrss[j] = mrss[j], mrss[i] }

func TestMockRpcSender(t *testing.T) {
	var err error
	mrs := newMockRpcSender()

	var actualReply *RpcAppendEntriesReply = nil
	var replyAsync func(*RpcAppendEntriesReply) = func(rpcReply *RpcAppendEntriesReply) {
		actualReply = rpcReply
	}

	err = mrs.SendRpcAppendEntriesAsync(
		"s2",
		&RpcAppendEntries{101, 8080, 100, nil, 8000},
		replyAsync,
	)
	if err != nil {
		t.Fatal(err)
	}
	err = mrs.SendRpcRequestVoteAsync(
		"s1",
		&RpcRequestVote{102, 8008, 100},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	expected := []mockSentRpc{
		{"s1", &RpcRequestVote{102, 8008, 100}},
		{"s2", &RpcAppendEntries{101, 8080, 100, nil, 8000}},
	}
	mrs.checkSentRpcs(t, expected)

	if actualReply != nil {
		t.Fatal()
	}

	sentReply := &RpcAppendEntriesReply{102, false}
	if mrs.sendReplies(sentReply) != 1 {
		t.Error()
	}
	expectedReply := RpcAppendEntriesReply{102, false}
	if actualReply == nil || *actualReply != expectedReply {
		t.Fatal(actualReply)
	}
}
