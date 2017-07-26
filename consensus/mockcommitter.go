package consensus

import (
	"fmt"
	"reflect"

	"github.com/divtxt/raft"
)

type mockCommitterCall struct {
	name  string
	param raft.LogIndex
	rc    chan raft.CommandResult
}

type mockCommitter struct {
	calls []mockCommitterCall
}

func newMockCommitter() *mockCommitter {
	return &mockCommitter{}
}

func (mc *mockCommitter) CheckCalls(expected []mockCommitterCall) {
	if len(mc.calls) == 0 && len(expected) == 0 {
		return
	}
	if !reflect.DeepEqual(mc.calls, expected) {
		panic(fmt.Sprintf("%v", mc.calls))
	}
	mc.calls = nil
}

func (mc *mockCommitter) RegisterListener(logIndex raft.LogIndex) <-chan raft.CommandResult {
	r := make(chan raft.CommandResult, 1)
	mc.calls = append(mc.calls, mockCommitterCall{"RegisterListener", logIndex, nil}) // r})
	return r
}

func (mc *mockCommitter) RemoveListenersAfterIndex(afterIndex raft.LogIndex) {
	mc.calls = append(mc.calls, mockCommitterCall{"RemoveListenersAfterIndex", afterIndex, nil})
}

func (mc *mockCommitter) CommitAsync(commitIndex raft.LogIndex) {
	mc.calls = append(mc.calls, mockCommitterCall{"CommitAsync", commitIndex, nil})
}
