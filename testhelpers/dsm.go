package testhelpers

import (
	"bytes"
	"fmt"
	"reflect"
	"strconv"

	. "github.com/divtxt/raft"
)

// Dummy state machine that implements StateMachine.
// Does not provide any useful state or commands. Meant only for tests.
type DummyStateMachine struct {
	lastApplied     LogIndex
	appliedCommands []Command
}

// Will serialize to Command("cN")
func DummyCommand(N int) Command {
	return Command("c" + strconv.Itoa(N))
}

func NewDummyStateMachine(lastApplied LogIndex) *DummyStateMachine {
	return &DummyStateMachine{
		lastApplied,
		[]Command{},
	}
}

func (dsm *DummyStateMachine) GetLastApplied() LogIndex {
	return dsm.lastApplied
}

func (dsm *DummyStateMachine) ApplyCommand(logIndex LogIndex, command Command) {
	if logIndex < dsm.lastApplied {
		panic(fmt.Sprintf(
			"DummyStateMachine: logIndex=%d is < current lastApplied=%d",
			logIndex,
			dsm.lastApplied,
		))
	}

	dsm.appliedCommands = append(dsm.appliedCommands, command)
	dsm.lastApplied = logIndex
}

func (dsm *DummyStateMachine) AppliedCommandsEqual(cmds ...int) bool {
	appliedCommands := make([]Command, len(cmds))
	for i, s := range cmds {
		appliedCommands[i] = DummyCommand(s)
	}
	return reflect.DeepEqual(dsm.appliedCommands, appliedCommands)
}

// Helper
func DummyCommandEquals(c Command, n int) bool {
	cn := Command("c" + strconv.Itoa(n))
	return bytes.Equal(c, cn)
}
