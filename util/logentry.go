package util

import (
	"github.com/divtxt/raft"
)

const (
	ET_UNUSED                raft.EntryType = 0 // Avoid using the zero value
	ET_STATE_MACHINE_COMMAND raft.EntryType = 1 // Command is meant for the StateMachine
	ET_NO_OP                 raft.EntryType = 2 // No-op entry - Command bytes should be empty
)

// Create a LogEntry for a state machine command.
func NewLogEntry(t raft.TermNo, c raft.Command) raft.LogEntry {
	return raft.LogEntry{t, ET_STATE_MACHINE_COMMAND, c}
}
