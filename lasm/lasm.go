package lasm

import (
	"fmt"
	. "github.com/divtxt/raft"
)

// Dummy in-memory implementation of LogAndStateMachine.
// Does not provide any useful state or commands. Meant only for tests,
type LogAndStateMachineImpl struct {
	log          Log
	stateMachine StateMachine
	commitIndex  LogIndex
}

func NewLogAndStateMachineImpl(log Log, stateMachine StateMachine) *LogAndStateMachineImpl {
	return &LogAndStateMachineImpl{log, stateMachine, 0}
}

func (lasmi *LogAndStateMachineImpl) GetIndexOfLastEntry() (LogIndex, error) {
	return lasmi.log.GetIndexOfLastEntry()
}

func (lasmi *LogAndStateMachineImpl) GetTermAtIndex(li LogIndex) (TermNo, error) {
	return lasmi.log.GetTermAtIndex(li)
}

func (lasmi *LogAndStateMachineImpl) SetEntriesAfterIndex(li LogIndex, entries []LogEntry) error {
	// Check that we're not trying to rewind past commitIndex
	if li < lasmi.commitIndex {
		return fmt.Errorf(
			"LogAndStateMachineImpl: setEntriesAfterIndex(%d, ...) but commitIndex=%d",
			li,
			lasmi.commitIndex,
		)
	}
	return lasmi.log.SetEntriesAfterIndex(li, entries)
}

func (lasmi *LogAndStateMachineImpl) AppendEntry(
	termNo TermNo,
	rawCommand interface{},
) (interface{}, error) {
	// Get the command checked and serialized
	command, reply, err := lasmi.stateMachine.ReviewAppendCommand(rawCommand)
	if err != nil {
		return nil, err
	}

	// If command rejected, return without appending to the log immediately.
	if command == nil {
		return reply, nil
	}

	// Append serialized command to the log.
	err = lasmi.log.AppendEntry(termNo, command)
	if err != nil {
		return nil, err
	}

	return reply, nil
}

func (lasmi *LogAndStateMachineImpl) GetEntriesAfterIndex(
	afterLogIndex LogIndex,
) ([]LogEntry, error) {
	return lasmi.log.GetEntriesAfterIndex(afterLogIndex)
}

func (lasmi *LogAndStateMachineImpl) CommitIndexChanged(commitIndex LogIndex) error {
	if commitIndex < lasmi.commitIndex {
		return fmt.Errorf(
			"LogAndStateMachineImpl: CommitIndexChanged(%d) is < current commitIndex=%d",
			commitIndex,
			lasmi.commitIndex,
		)
	}
	iole, err := lasmi.GetIndexOfLastEntry()
	if err != nil {
		return err
	}
	if commitIndex > iole {
		return fmt.Errorf(
			"LogAndStateMachineImpl: CommitIndexChanged(%d) is > iole=%d",
			commitIndex,
			iole,
		)
	}
	lasmi.commitIndex = commitIndex
	return nil
}

func (lasmi *LogAndStateMachineImpl) GetCommitIndex() LogIndex {
	return lasmi.commitIndex
}
