package testhelpers

import (
	"errors"
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
	c                  chan MockSentRpc
	processReplyAsyncs chan func(interface{})
}

type MockSentRpc struct {
	ToServer ServerId
	Rpc      interface{}
}

func NewMockRpcSender() *MockRpcSender {
	return &MockRpcSender{
		make(chan MockSentRpc, 100),
		make(chan func(interface{}), 100),
	}
}

func (mrs *MockRpcSender) SendOnlyRpcAppendEntriesAsync(
	toServer ServerId,
	rpc *RpcAppendEntries,
) error {
	return mrs.SendRpcAppendEntriesAsync(toServer, rpc, nil)
}

func (mrs *MockRpcSender) SendOnlyRpcRequestVoteAsync(
	toServer ServerId,
	rpc *RpcRequestVote,
) error {
	return mrs.SendRpcRequestVoteAsync(toServer, rpc, nil)
}

func (mrs *MockRpcSender) SendRpcAppendEntriesAsync(
	toServer ServerId,
	rpc *RpcAppendEntries,
	processReplyAsync func(*RpcAppendEntriesReply),
) error {
	select {
	default:
		return errors.New("oops!")
	case mrs.c <- MockSentRpc{toServer, rpc}:
		if processReplyAsync != nil {
			mrs.processReplyAsyncs <- func(rpcReply interface{}) {
				switch rpcReply := rpcReply.(type) {
				case *RpcAppendEntriesReply:
					processReplyAsync(rpcReply)
				default:
					panic("oops!")
				}
			}
		}
	}
	return nil
}

func (mrs *MockRpcSender) SendRpcRequestVoteAsync(
	toServer ServerId,
	rpc *RpcRequestVote,
	processReplyAsync func(*RpcRequestVoteReply),
) error {
	select {
	default:
		return errors.New("oops!")
	case mrs.c <- MockSentRpc{toServer, rpc}:
		if processReplyAsync != nil {
			mrs.processReplyAsyncs <- func(rpcReply interface{}) {
				switch rpcReply := rpcReply.(type) {
				case *RpcRequestVoteReply:
					processReplyAsync(rpcReply)
				default:
					panic("oops!")
				}
			}
		}
	}
	return nil
}

// Clear sent rpcs.
func (mrs *MockRpcSender) ClearSentRpcs() {
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
func (mrs *MockRpcSender) CheckSentRpcs(t *testing.T, expectedRpcs []MockSentRpc) {
	rpcs := make([]MockSentRpc, 0, 100)

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
func (mrs *MockRpcSender) SendReplies(reply interface{}) int {
	var n int = 0
loop:
	for {
		select {
		case processReplyAsync := <-mrs.processReplyAsyncs:
			processReplyAsync(reply)
			n++
		default:
			break loop
		}
	}
	return n
}

// implement sort.Interface for MockSentRpc slices
type MockRpcSenderSlice []MockSentRpc

func (mrss MockRpcSenderSlice) Len() int           { return len(mrss) }
func (mrss MockRpcSenderSlice) Less(i, j int) bool { return mrss[i].ToServer < mrss[j].ToServer }
func (mrss MockRpcSenderSlice) Swap(i, j int)      { mrss[i], mrss[j] = mrss[j], mrss[i] }
