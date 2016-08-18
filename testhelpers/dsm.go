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
type DummyCommand struct {
	N              int
	RejectMePlease bool
}

const (
	DummyCommand_Reply_Ok     = "We accept your offering!"
	DummyCommand_Reply_Reject = "This is unacceptable!"
)

func NewDummyStateMachine() *DummyStateMachine {
	return &DummyStateMachine{0}
}

func (dsm *DummyStateMachine) ReviewAppendCommand(
	rawCommand interface{},
) (Command, interface{}, error) {

	switch rawCommand := rawCommand.(type) {
	case DummyCommand:
		if rawCommand.RejectMePlease {
			return nil, DummyCommand_Reply_Reject, nil
		} else {
			command := Command("c" + strconv.Itoa(rawCommand.N))
			return command, DummyCommand_Reply_Ok, nil
		}
	default:
		err := fmt.Errorf("oops! rawCommand %#v of unknown type: %T", rawCommand, rawCommand)
		return nil, nil, err
	}
}

func (dsm *DummyStateMachine) CommitIndexChanged(commitIndex LogIndex) error {
	if commitIndex < dsm.commitIndex {
		// LogAndStateMachine should prevent this from happening!
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
