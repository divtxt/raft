package util

import (
	"fmt"

	"github.com/divtxt/raft"
)

type CommitNotifier struct {
	commitIndex  raft.LogIndex
	highestIndex raft.LogIndex
	listeners    map[raft.LogIndex]chan struct{}
}

func NewCommitNotifier(initialCommitIndex raft.LogIndex) *CommitNotifier {
	return &CommitNotifier{
		initialCommitIndex,
		0,
		make(map[raft.LogIndex]chan struct{}),
	}
}

func (cn *CommitNotifier) GetCommitIndex() raft.LogIndex {
	return cn.commitIndex
}

func (cn *CommitNotifier) RegisterListener(logIndex raft.LogIndex) raft.CommitSignal {
	if logIndex <= cn.commitIndex {
		panic(fmt.Sprintf("FATAL: logIndex=%v is <= commitIndex=%v", logIndex, cn.commitIndex))
	}

	l := make(chan struct{}, 1)
	if _, ok := cn.listeners[logIndex]; ok {
		panic(fmt.Sprintf("FATAL: replyChan already exists for logIndex=%v", logIndex))
	}
	cn.listeners[logIndex] = l

	if logIndex > cn.highestIndex {
		cn.highestIndex = logIndex
	}

	return l
}

func (cn *CommitNotifier) CommitIndexChanged(newCommitIndex raft.LogIndex) {
	if newCommitIndex < cn.commitIndex {
		panic(fmt.Sprintf(
			"FATAL: newCommitIndex=%v is < current commitIndex=%v", newCommitIndex, cn.commitIndex,
		))
	}

	for i := cn.commitIndex + 1; i <= newCommitIndex; i++ {
		l, ok := cn.listeners[i]
		if ok {
			delete(cn.listeners, i)
			l <- struct{}{}
		}
	}
	cn.commitIndex = newCommitIndex
}

func (cn *CommitNotifier) ClearListenersAfterIndex(afterIndex raft.LogIndex) {
	for i := afterIndex + 1; i <= cn.highestIndex; i++ {
		l, ok := cn.listeners[i]
		if ok {
			delete(cn.listeners, i)
			close(l)
		}
	}
	cn.highestIndex = afterIndex
}
