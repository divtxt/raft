package testhelpers

import (
	"fmt"
	. "github.com/divtxt/raft"
	"reflect"
	"sort"
	"testing"
)

// -- rpcSender

// Mock in-memory implementation of both RpcService & RpcSendOnly
// - meant only for tests
type MockRpcSender struct {
	sentRpcs         []MockSentRpc
	sendAEReplyFuncs []func(*RpcAppendEntriesReply)
	sendRVReplyFuncs []func(*RpcRequestVoteReply)
}

type MockSentRpc struct {
	ToServer ServerId
	Rpc      interface{}
}

func NewMockRpcSender() *MockRpcSender {
	return &MockRpcSender{nil, nil, nil}
}

func (mrs *MockRpcSender) SendOnlyRpcAppendEntriesAsync(
	toServer ServerId,
	rpc *RpcAppendEntries,
) error {
	mrs.sentRpcs = append(mrs.sentRpcs, MockSentRpc{toServer, rpc})
	return nil
}

func (mrs *MockRpcSender) SendOnlyRpcRequestVoteAsync(
	toServer ServerId,
	rpc *RpcRequestVote,
) error {
	mrs.sentRpcs = append(mrs.sentRpcs, MockSentRpc{toServer, rpc})
	return nil
}

func (mrs *MockRpcSender) RpcAppendEntries(
	toServer ServerId,
	rpc *RpcAppendEntries,
) *RpcAppendEntriesReply {
	replyChan := make(chan *RpcAppendEntriesReply)

	mrs.sentRpcs = append(mrs.sentRpcs, MockSentRpc{toServer, rpc})
	mrs.sendAEReplyFuncs = append(mrs.sendAEReplyFuncs, func(reply *RpcAppendEntriesReply) {
		replyChan <- reply
	})

	return <-replyChan
}

func (mrs *MockRpcSender) RpcRequestVote(
	toServer ServerId,
	rpc *RpcRequestVote,
) *RpcRequestVoteReply {
	replyChan := make(chan *RpcRequestVoteReply)

	mrs.sentRpcs = append(mrs.sentRpcs, MockSentRpc{toServer, rpc})
	mrs.sendRVReplyFuncs = append(mrs.sendRVReplyFuncs, func(reply *RpcRequestVoteReply) {
		replyChan <- reply
	})

	return <-replyChan
}

// Clear sent rpcs.
func (mrs *MockRpcSender) ClearSentRpcs() {
	mrs.sentRpcs = nil
	mrs.sendAEReplyFuncs = nil
	mrs.sendRVReplyFuncs = nil
}

// Clears & checks sent rpcs.
// expectedRpcs should be sorted by server
func (mrs *MockRpcSender) CheckSentRpcs(t *testing.T, expectedRpcs []MockSentRpc) {
	rpcs := mrs.sentRpcs
	mrs.sentRpcs = nil

	sort.Sort(MockRpcSenderSlice(rpcs))

	diffs := false
	if len(rpcs) == len(expectedRpcs) {
		for i := 0; i < len(rpcs); i++ {
			if !reflect.DeepEqual(rpcs[i], expectedRpcs[i]) {
				t.Error(fmt.Sprintf(
					"diff at [%v] - expected: [{%v %v}]; got: [{%v %v}]",
					i,
					expectedRpcs[i].ToServer, expectedRpcs[i].Rpc,
					rpcs[i].ToServer, rpcs[i].Rpc,
				))
				diffs = true
			}
		}
	} else {
		t.Error(fmt.Sprintf("Expected len: %v; got len: %v", len(expectedRpcs), len(rpcs)))
		diffs = true
	}
	if diffs {
		t.Error(fmt.Sprintf("Expected: %#v", expectedRpcs))
		t.Error(fmt.Sprintf("Got: %#v", rpcs))
		t.Fatal("Sadness :P")
	}
}

// Clears & sends reply to sent reply functions
func (mrs *MockRpcSender) SendAEReplies(reply *RpcAppendEntriesReply) int {
	replyFuncs := mrs.sendAEReplyFuncs
	mrs.sendAEReplyFuncs = nil

	for _, f := range replyFuncs {
		f(reply)
	}

	return len(replyFuncs)
}

func (mrs *MockRpcSender) SendRVReplies(reply *RpcRequestVoteReply) int {
	replyFuncs := mrs.sendRVReplyFuncs
	mrs.sendRVReplyFuncs = nil

	for _, f := range replyFuncs {
		f(reply)
	}

	return len(replyFuncs)
}

// implement sort.Interface for MockSentRpc slices
type MockRpcSenderSlice []MockSentRpc

func (mrss MockRpcSenderSlice) Len() int           { return len(mrss) }
func (mrss MockRpcSenderSlice) Less(i, j int) bool { return mrss[i].ToServer < mrss[j].ToServer }
func (mrss MockRpcSenderSlice) Swap(i, j int)      { mrss[i], mrss[j] = mrss[j], mrss[i] }
