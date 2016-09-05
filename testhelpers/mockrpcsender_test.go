package testhelpers

import (
	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/testdata"
	"testing"
	"time"
)

func TestMockRpcSender(t *testing.T) {
	mrs := NewMockRpcSender()

	var actualReply *RpcAppendEntriesReply = nil

	go func() {
		actualReply = mrs.RpcAppendEntries(
			"s2",
			&RpcAppendEntries{101, 8080, 100, nil, 8000},
		)
	}()

	go func() {
		mrs.RpcRequestVote(
			"s1",
			&RpcRequestVote{102, 8008, 100},
		)
	}()

	time.Sleep(testdata.SleepToLetGoroutineRun)

	expected := map[ServerId]interface{}{
		"s1": &RpcRequestVote{102, 8008, 100},
		"s2": &RpcAppendEntries{101, 8080, 100, nil, 8000},
	}
	mrs.CheckSentRpcs(t, expected)

	if actualReply != nil {
		t.Fatal()
	}

	sentReply := &RpcAppendEntriesReply{102, false}
	if mrs.SendAERepliesAndClearRpcs(sentReply) != 1 {
		t.Error()
	}

	time.Sleep(testdata.SleepToLetGoroutineRun)

	expectedReply := RpcAppendEntriesReply{102, false}
	if actualReply == nil || *actualReply != expectedReply {
		t.Fatal(actualReply)
	}
}
