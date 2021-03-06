package testhelpers

import (
	"github.com/divtxt/raft"
)

func AssertWillBlock(cs <-chan raft.CommandResult) {
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

func GetCommandResult(cs <-chan raft.CommandResult) raft.CommandResult {
	select {
	case v, ok := <-cs:
		if !ok {
			panic("channel should have value but is closed")
		}
		return v
	default:
		panic("channel should have value but does not")
	}
}
func AssertIsClosed(cs <-chan raft.CommandResult) {
	select {
	case _, ok := <-cs:
		if ok {
			panic("channel should be closed but has value ")
		}
	default:
		panic("channel should be closed but is not")
	}
}
