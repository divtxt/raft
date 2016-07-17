package testhelpers

import (
	. "github.com/divtxt/raft"
	"testing"
)

func TestMockRpcSender(t *testing.T) {
	var err error
	mrs := NewMockRpcSender()

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

	expected := []MockSentRpc{
		{"s1", &RpcRequestVote{102, 8008, 100}},
		{"s2", &RpcAppendEntries{101, 8080, 100, nil, 8000}},
	}
	mrs.CheckSentRpcs(t, expected)

	if actualReply != nil {
		t.Fatal()
	}

	sentReply := &RpcAppendEntriesReply{102, false}
	if mrs.SendReplies(sentReply) != 1 {
		t.Error()
	}
	expectedReply := RpcAppendEntriesReply{102, false}
	if actualReply == nil || *actualReply != expectedReply {
		t.Fatal(actualReply)
	}
}
