package raft

import (
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

	// Set & get tests
	persistentState.SetCurrentTermAndVotedFor(1, "s1")
	if persistentState.GetCurrentTerm() != 1 {
		t.Fatal()
	}
	if persistentState.GetVotedFor() != "s1" {
		t.Fatal()
	}
	persistentState.SetCurrentTermAndVotedFor(4, "s2")
	if persistentState.GetCurrentTerm() != 4 {
		t.Fatal()
	}
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

func (imps *inMemoryPersistentState) SetCurrentTermAndVotedFor(
	currentTerm TermNo,
	votedFor ServerId,
) {
	imps.mutex.Lock()
	defer imps.mutex.Unlock()
	imps.currentTerm = currentTerm
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

// -- RpcSender

// Mock in-memory implementation of RpcSender - meant only for tests
type mockRpcSender struct {
	c chan mockSentRpc
}

type mockSentRpc struct {
	toServer ServerId
	rpc      interface{}
}

func newMockRpcSender() *mockRpcSender {
	return &mockRpcSender{make(chan mockSentRpc, 100)}
}

func (mrs *mockRpcSender) SendAsync(toServer ServerId, rpc interface{}) {
	select {
	default:
		panic("oops!")
	case mrs.c <- mockSentRpc{toServer, rpc}:
		// nothing more to do!
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

	if !reflect.DeepEqual(rpcs, expectedRpcs) {
		t.Fatal(rpcs)
	}
}

// implement sort.Interface for mockSentRpc slices
type mockRpcSenderSlice []mockSentRpc

func (mrss mockRpcSenderSlice) Len() int           { return len(mrss) }
func (mrss mockRpcSenderSlice) Less(i, j int) bool { return mrss[i].toServer < mrss[j].toServer }
func (mrss mockRpcSenderSlice) Swap(i, j int)      { mrss[i], mrss[j] = mrss[j], mrss[i] }

func TestMockRpcSender(t *testing.T) {
	mrs := newMockRpcSender()

	mrs.SendAsync("s2", "foo")
	mrs.SendAsync("s1", 42)

	expected := []mockSentRpc{{"s1", 42}, {"s2", "foo"}}
	mrs.checkSentRpcs(t, expected)
}
