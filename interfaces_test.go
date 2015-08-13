package raft

import (
	"reflect"
	"sync"
	"testing"
)

// PersistentState blackbox test
// Send a PersistentState in new / reset state.
func PersistentStateBlackboxTest(t *testing.T, persistentState PersistentState) {
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
	PersistentStateBlackboxTest(t, imps)
}

// Mock in-memory implementation of RpcSender - meant only for tests
type mockRpcSender struct {
	c chan mockSentRpc
}

type mockSentRpc struct {
	rpc      interface{}
	toServer ServerId
}

func newMockRpcSender() *mockRpcSender {
	return &mockRpcSender{make(chan mockSentRpc, 100)}
}

func (mrs *mockRpcSender) SendAsync(rpc interface{}, toServer ServerId) {
	select {
	default:
		panic("oops!")
	case mrs.c <- mockSentRpc{rpc, toServer}:
		// nothing more to do!
	}
}

func TestMockRpcSender(t *testing.T) {
	mrs := newMockRpcSender()

	mrs.SendAsync("foo", "s1")
	mrs.SendAsync(42, "s2")

	actual := make([]mockSentRpc, 0, 2)
loop:
	for {
		select {
		case v := <-mrs.c:
			n := len(actual)
			actual = actual[0 : n+1]
			actual[n] = v
		default:
			break loop
		}
	}

	expected := []mockSentRpc{mockSentRpc{"foo", "s1"}, mockSentRpc{42, "s2"}}

	if !reflect.DeepEqual(actual, expected) {
		t.Fatal()
	}
}
