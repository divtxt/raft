package testhelpers

import (
	"bytes"
	"fmt"
	. "github.com/divtxt/raft"
	"strconv"
)

// Dummy in-memory implementation of StateMachine.
// Does not provide any useful state or commands. Meant only for tests.
type DummyStateMachine struct {
	commitIndex LogIndex
}

// Will serialize to Command("cN")
func DummyCommand(N int) Command {
	return Command("c" + strconv.Itoa(N))
}

func NewDummyStateMachine() *DummyStateMachine {
	return &DummyStateMachine{0}
}

func (dsm *DummyStateMachine) CommitIndexChanged(commitIndex LogIndex) error {
	if commitIndex < dsm.commitIndex {
		// Panic instead of returning error here because we expect to never get here in tests.
		panic(fmt.Sprintf(
			"DummyStateMachine: CommitIndexChanged(%d) is < current commitIndex=%d",
			commitIndex,
			dsm.commitIndex,
		))
	}
	dsm.commitIndex = commitIndex
	return nil
}

func (dsm *DummyStateMachine) GetCommitIndex() LogIndex {
	return dsm.commitIndex
}

// Helper
func DummyCommandEquals(c Command, n int) bool {
	cn := Command("c" + strconv.Itoa(n))
	return bytes.Equal(c, cn)
}
