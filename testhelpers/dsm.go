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
	commitIndex     LogIndex
	appliedCommands []string
}

// Will serialize to Command("cN")
func DummyCommand(N int) Command {
	return Command("c" + strconv.Itoa(N))
}

func DummyResponse(cmd Command) CommandResponse {
	return "r" + string(cmd)
}

func newDummyStateMachine(lastApplied LogIndex) *DummyStateMachine {
	return &DummyStateMachine{
		lastApplied,
		[]string{},
	}
}

func NewDummyStateMachineFromLog(lastApplied LogIndex, lro LogReadOnly) *DummyStateMachine {
	l, err := lro.GetIndexOfLastEntry()
	if err != nil {
		panic(err)
	}
	if l == 0 {
		return newDummyStateMachine(lastApplied)
	}

	entries, err := lro.GetEntriesAfterIndex(0, uint64(l))
	if err != nil {
		panic(err)
	}

	commands := make([]string, 0, l)

	for _, e := range entries {
		commands = append(commands, string(e.Command))
	}

	return &DummyStateMachine{
		lastApplied,
		commands,
	}
}

func (dsm *DummyStateMachine) GetLastApplied() LogIndex {
	return dsm.commitIndex
}

func (dsm *DummyStateMachine) CheckAndApplyCommand(
	logIndex LogIndex, command Command,
) (CommandResponse, error) {
	if logIndex < dsm.commitIndex {
		panic(fmt.Sprintf(
			"DummyStateMachine: logIndex=%d is < current commitIndex=%d",
			logIndex,
			dsm.commitIndex,
		))
	}
	indexOfLastEntry := len(dsm.appliedCommands)
	if int(logIndex) != 1+indexOfLastEntry {
		panic(fmt.Sprintf(
			"DummyStateMachine: logIndex=%d is != 1 + indexOfLastEntry=%d",
			logIndex,
			indexOfLastEntry,
		))
	}

	// reject "invalid" command
	n, err := strconv.Atoi(string(command[1:]))
	if err != nil {
		panic(err)
	}
	if n <= 0 {
		return nil, fmt.Errorf("Invalid command: %s", command)
	}

	dsm.appliedCommands = append(dsm.appliedCommands, string(command))
	return DummyResponse(command), nil
}

func (dsm *DummyStateMachine) SetEntriesAfterIndex(logIndex LogIndex, entries []LogEntry) error {
	if logIndex < dsm.commitIndex {
		return fmt.Errorf(
			"DummyStateMachine: logIndex=%d is < current commitIndex=%d",
			logIndex,
			dsm.commitIndex,
		)
	}

	// discard if needed
	if int(logIndex) < len(dsm.appliedCommands) {
		dsm.appliedCommands = dsm.appliedCommands[0:logIndex]
	}

	// append new
	for _, e := range entries {
		logIndex++
		_, err := dsm.CheckAndApplyCommand(logIndex, e.Command)
		if err != nil {
			panic(err)
		}
	}

	return nil
}

func (dsm *DummyStateMachine) AppliedCommandsEqual(cmds ...int) bool {
	appliedCommands := make([]string, len(cmds))
	for i, s := range cmds {
		appliedCommands[i] = string(DummyCommand(s))
	}
	return reflect.DeepEqual(dsm.appliedCommands, appliedCommands)
}

func (dsm *DummyStateMachine) CommitIndexChanged(commitIndex LogIndex) {
	if commitIndex < dsm.commitIndex {
		panic(fmt.Sprintf(
			"MockCommitIndexChangeListener: CommitIndexChanged(%d) is < current commitIndex=%d",
			commitIndex,
			dsm.commitIndex,
		))
	}
	dsm.commitIndex = commitIndex
}

// Helper
func DummyCommandEquals(c Command, n int) bool {
	cn := Command("c" + strconv.Itoa(n))
	return bytes.Equal(c, cn)
}
