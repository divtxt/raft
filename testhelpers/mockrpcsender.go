package testhelpers

import (
	"fmt"
	. "github.com/divtxt/raft"
	"reflect"
	"sync"
	"testing"
)

// Mock in-memory implementation of both RpcService and RpcSendOnly
// - meant only for tests
type MockRpcSender struct {
	mutex    *sync.Mutex
	sentRpcs map[ServerId]interface{}
}

type SentAppendEntries struct {
	Rpc       *RpcAppendEntries
	ReplyChan chan *RpcAppendEntriesReply
}
type SentRequestVote struct {
	Rpc       *RpcRequestVote
	ReplyChan chan *RpcRequestVoteReply
}

func NewMockRpcSender() *MockRpcSender {
	return &MockRpcSender{
		&sync.Mutex{},
		make(map[ServerId]interface{}),
	}
}

// RpcSendOnly implementation

func (mrs *MockRpcSender) SendOnlyRpcAppendEntriesAsync(
	toServer ServerId,
	rpc *RpcAppendEntries,
) {
	mrs.sendRpc(toServer, SentAppendEntries{rpc, nil})
}

func (mrs *MockRpcSender) SendOnlyRpcRequestVoteAsync(
	toServer ServerId,
	rpc *RpcRequestVote,
) {
	mrs.sendRpc(toServer, SentRequestVote{rpc, nil})
}

// RpcService implementation

func (mrs *MockRpcSender) RpcAppendEntries(
	toServer ServerId,
	rpc *RpcAppendEntries,
) *RpcAppendEntriesReply {
	replyChan := make(chan *RpcAppendEntriesReply)
	mrs.sendRpc(toServer, SentAppendEntries{rpc, replyChan})
	return <-replyChan
}

func (mrs *MockRpcSender) RpcRequestVote(
	toServer ServerId,
	rpc *RpcRequestVote,
) *RpcRequestVoteReply {
	replyChan := make(chan *RpcRequestVoteReply)
	mrs.sendRpc(toServer, SentRequestVote{rpc, replyChan})
	return <-replyChan
}

func (mrs *MockRpcSender) sendRpc(toServer ServerId, sentRpc interface{}) {
	mrs.mutex.Lock()
	defer mrs.mutex.Unlock()

	if _, hasKey := mrs.sentRpcs[toServer]; hasKey {
		panic(fmt.Sprintf("Already have sent rpc for server %v", toServer))
	}
	mrs.sentRpcs[toServer] = sentRpc
}

func (mrs *MockRpcSender) CheckSentRpcs(t *testing.T, expectedRpcs map[ServerId]interface{}) {
	mrs.mutex.Lock()
	defer mrs.mutex.Unlock()

	if len(mrs.sentRpcs) != len(expectedRpcs) {
		mrs.fatalSentRpcs(t, expectedRpcs)
	}

	for toServer, expectedRpc := range expectedRpcs {
		sentRpc, hasKey := mrs.sentRpcs[toServer]
		if !hasKey {
			mrs.fatalSentRpcs(t, expectedRpcs)
		}
		if !rpcEquals(sentRpc, expectedRpc) {
			mrs.fatalSentRpcs(t, expectedRpcs)
		}
	}
}

func rpcEquals(sentRpc interface{}, rpc interface{}) bool {
	switch sentRpc := sentRpc.(type) {
	case SentAppendEntries:
		return reflect.DeepEqual(sentRpc.Rpc, rpc)
	case SentRequestVote:
		return reflect.DeepEqual(sentRpc.Rpc, rpc)
	default:
		panic("oops")
	}
}

func (mrs *MockRpcSender) fatalSentRpcs(t *testing.T, expectedRpcs map[ServerId]interface{}) {
	t.Error("--- Sent:")
	for toServer, sentRpc := range mrs.sentRpcs {
		switch sentRpc := sentRpc.(type) {
		case SentAppendEntries:
			t.Error(fmt.Sprintf("toServer: %v - RpcAppendEntries: %v", toServer, sentRpc.Rpc))
		case SentRequestVote:
			t.Error(fmt.Sprintf("toServer: %v - RpcRequestVote: %v", toServer, sentRpc.Rpc))
		default:
			t.Errorf("toServer: %v - %T: %#v", toServer, sentRpc, sentRpc)

		}
	}

	t.Error("--- Expected:")
	for toServer, rpc := range expectedRpcs {
		switch rpc := rpc.(type) {
		case *RpcAppendEntries:
			t.Error(fmt.Sprintf("toServer: %v - RpcAppendEntries: %v", toServer, rpc))
		case *RpcRequestVote:
			t.Error(fmt.Sprintf("toServer: %v - RpcRequestVote: %v", toServer, rpc))
		default:
			t.Errorf("toServer: %v - %T: %v", toServer, rpc, rpc)
		}
	}

	panic("Sadness :(")
}

func (mrs *MockRpcSender) SendAERepliesAndClearRpcs(reply *RpcAppendEntriesReply) int {
	return mrs.sendRepliesAndClearRpcs(reply, nil)
}

func (mrs *MockRpcSender) SendRVRepliesAndClearRpcs(reply *RpcRequestVoteReply) int {
	return mrs.sendRepliesAndClearRpcs(nil, reply)
}

func (mrs *MockRpcSender) sendRepliesAndClearRpcs(
	aeReply *RpcAppendEntriesReply,
	rvReply *RpcRequestVoteReply,
) int {
	mrs.mutex.Lock()
	defer mrs.mutex.Unlock()

	n := 0
	for _, sentRpc := range mrs.sentRpcs {
		switch sentRpc := sentRpc.(type) {
		case SentAppendEntries:
			if aeReply != nil {
				sentRpc.ReplyChan <- aeReply
				n++
			}
		case SentRequestVote:
			if rvReply != nil {
				sentRpc.ReplyChan <- rvReply
				n++
			}
		}
	}

	mrs.sentRpcs = make(map[ServerId]interface{})
	return n
}

func (mrs *MockRpcSender) ClearSentRpcs() {
	mrs.mutex.Lock()
	defer mrs.mutex.Unlock()

	mrs.sentRpcs = make(map[ServerId]interface{})
}
