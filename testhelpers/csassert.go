package testhelpers

import (
	"github.com/divtxt/raft"
)

func AssertWillBlock(cs raft.CommitSignal) {
	select {
	case _, ok := <-cs:
		if ok {
			panic("channel should block but has value ")
		} else {
			panic("channel should block but is closed")
		}
	default:
	}
}

func AssertHasValue(cs raft.CommitSignal) {
	select {
	case _, ok := <-cs:
		if !ok {
			panic("channel should have value but is closed")
		}
	default:
		panic("channel should have value but does not")
	}
}

func AssertClosed(cs raft.CommitSignal) {
	select {
	case _, ok := <-cs:
		if ok {
			panic("channel should be closed but has value ")
		}
	default:
		panic("channel should be closed but is not")
	}
}
