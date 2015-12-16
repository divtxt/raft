package raft

import (
	"fmt"
	"reflect"
	"sort"
	"sync"
	"testing"
)

// -- PersistentState

// PersistentState blackbox test.
// Send a PersistentState in new / reset state.
func PartialTest_PersistentState_BlackboxTest(t *testing.T, persistentState PersistentState) {
	// Initial data tests
	if persistentState.GetCurrentTerm() != 0 {
		t.Fatal()
	}
	if persistentState.GetVotedFor() != "" {
		t.Fatal()
	}

	// Set currentTerm to 0 is an error
	test_ExpectPanic(
		t,
		func() {
			persistentState.SetCurrentTerm(0)
		},
		"FATAL: attempt to set currentTerm to 0",
	)
	if persistentState.GetCurrentTerm() != 0 {
		t.Fatal()
	}
	// Set votedFor while currentTerm is 0 is an error
	test_ExpectPanic(
		t,
		func() {
			persistentState.SetVotedFor("s1")
		},
		"FATAL: attempt to set votedFor while currentTerm is 0",
	)
	if persistentState.GetCurrentTerm() != 0 {
		t.Fatal()
	}
	// Set currentTerm greater is ok, clears votedFor
	persistentState.SetCurrentTerm(1)
	if persistentState.GetCurrentTerm() != 1 {
		t.Fatal()
	}
	// Set votedFor of blank is an error
	test_ExpectPanic(
		t,
		func() {
			persistentState.SetVotedFor("")
		},
		"FATAL: attempt to set blank votedFor",
	)
	if persistentState.GetVotedFor() != "" {
		t.Fatal()
	}
	// Set votedFor is ok
	persistentState.SetVotedFor("s1")
	if persistentState.GetVotedFor() != "s1" {
		t.Fatal()
	}
	// Set currentTerm greater is ok, clears votedFor
	persistentState.SetCurrentTerm(4)
	if persistentState.GetCurrentTerm() != 4 {
		t.Fatal()
	}
	if persistentState.GetVotedFor() != "" {
		t.Fatal(persistentState.GetVotedFor())
	}
	// Set votedFor while blank is ok
	persistentState.SetVotedFor("s2")
	if persistentState.GetVotedFor() != "s2" {
		t.Fatal()
	}
	// Set currentTerm same is ok, does not affect votedFor
	persistentState.SetCurrentTerm(4)
	if persistentState.GetCurrentTerm() != 4 {
		t.Fatal()
	}
	if persistentState.GetVotedFor() != "s2" {
		t.Fatal()
	}
	// Set currentTerm less is an error
	test_ExpectPanic(
		t,
		func() {
			persistentState.SetCurrentTerm(3)
		},
		"FATAL: attempt to decrease currentTerm: 4 to 3",
	)
	if persistentState.GetCurrentTerm() != 4 {
		t.Fatal()
	}
	// Set votedFor while not blank is an error
	test_ExpectPanic(
		t,
		func() {
			persistentState.SetVotedFor("s3")
		},
		"FATAL: attempt to change non-blank votedFor: s2 to s3",
	)
	if persistentState.GetVotedFor() != "s2" {
		t.Fatal()
	}
}

// In-memory implementation of PersistentState - meant only for tests
type inMemoryPersistentState struct {
	mutex       *sync.Mutex
	currentTerm TermNo
	votedFor    ServerId
}

func (imps *inMemoryPersistentState) GetCurrentTerm() TermNo {
	imps.mutex.Lock()
	defer imps.mutex.Unlock()
	return imps.currentTerm
}

func (imps *inMemoryPersistentState) GetVotedFor() ServerId {
	imps.mutex.Lock()
	defer imps.mutex.Unlock()
	return imps.votedFor
}

func (imps *inMemoryPersistentState) SetCurrentTerm(currentTerm TermNo) {
	imps.mutex.Lock()
	defer imps.mutex.Unlock()
	if currentTerm == 0 {
		panic("FATAL: attempt to set currentTerm to 0")
	}
	if currentTerm < imps.currentTerm {
		panic(fmt.Sprintf("FATAL: attempt to decrease currentTerm: %v to %v", imps.currentTerm, currentTerm))
	}
	if currentTerm > imps.currentTerm {
		imps.votedFor = ""
	}
	imps.currentTerm = currentTerm
}

func (imps *inMemoryPersistentState) SetVotedFor(votedFor ServerId) {
	imps.mutex.Lock()
	defer imps.mutex.Unlock()
	if imps.currentTerm == 0 {
		panic("FATAL: attempt to set votedFor while currentTerm is 0")
	}
	if votedFor == "" {
		panic("FATAL: attempt to set blank votedFor")
	}
	if imps.votedFor != "" {
		panic(fmt.Sprintf("FATAL: attempt to change non-blank votedFor: %v to %v", imps.votedFor, votedFor))
	}
	imps.votedFor = votedFor
}

func newIMPSWithCurrentTerm(currentTerm TermNo) *inMemoryPersistentState {
	return &inMemoryPersistentState{&sync.Mutex{}, currentTerm, ""}
}

// Run the blackbox test on inMemoryPersistentState
func TestInMemoryPersistentState(t *testing.T) {
	imps := newIMPSWithCurrentTerm(0)
	PartialTest_PersistentState_BlackboxTest(t, imps)
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

func (mrs *mockRpcSender) sendAsync(toServer ServerId, rpc interface{}) {
	mrs.SendAsync(toServer, rpc, nil)
}

func (mrs *mockRpcSender) SendAsync(toServer ServerId, rpc interface{}, replyAsync func(interface{})) {
	select {
	default:
		panic("oops!")
	case mrs.c <- mockSentRpc{toServer, rpc}:
		if replyAsync != nil {
			mrs.replyAsyncs <- replyAsync
		}
	}
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
	mrs := newMockRpcSender()

	var actualReply interface{} = nil
	var replyAsync func(interface{}) = func(rpcReply interface{}) {
		actualReply = rpcReply
	}

	mrs.SendAsync("s2", "foo", replyAsync)
	mrs.sendAsync("s1", 42)

	expected := []mockSentRpc{{"s1", 42}, {"s2", "foo"}}
	mrs.checkSentRpcs(t, expected)

	if actualReply != nil {
		t.Fatal()
	}

	sentReply := &struct{ int }{24}
	if mrs.sendReplies(sentReply) != 1 {
		t.Error()
	}

	if !reflect.DeepEqual(actualReply, sentReply) {
		t.Fatal()
	}
}
