package raft

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"testing"
)

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
