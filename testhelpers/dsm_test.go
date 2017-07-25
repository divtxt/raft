package testhelpers

import (
	"testing"

	. "github.com/divtxt/raft"
)

// Test for implementations of the StateMachine interface.
//
// Send a Log with 10 entries with terms as shown in Figure 7, leader line.
// Entries should be Command("c1"), Command("c2"), etc.
func TestDummyStateMachine(t *testing.T) {
	dsm := newDummyStateMachine(0)
	var sm StateMachine = dsm

	// Initial data tests
	if sm.GetLastApplied() != 0 {
		t.Fatal(dsm)
	}
	if !dsm.AppliedCommandsEqual() {
		t.Fatal(dsm)
	}

	// Append some commands
	err := sm.CheckAndApplyCommand(1, DummyCommand(101))
	if err != nil {
		t.Fatal(err)
	}
	err = sm.CheckAndApplyCommand(2, DummyCommand(102))
	if err != nil {
		t.Fatal(err)
	}
	err = sm.CheckAndApplyCommand(3, DummyCommand(103))
	if err != nil {
		t.Fatal(err)
	}
	if !dsm.AppliedCommandsEqual(101, 102, 103) {
		t.Fatal(dsm)
	}

	// Append rejection
	err = sm.CheckAndApplyCommand(4, DummyCommand(-1))
	if err.Error() != "Invalid command: c-1" {
		t.Fatal(err)
	}
	if !dsm.AppliedCommandsEqual(101, 102, 103) {
		t.Fatal(dsm)
	}

	// Bad append
	TestHelper_ExpectPanicMessage(
		t,
		func() {
			_ = sm.CheckAndApplyCommand(3, DummyCommand(104))
		},
		"DummyStateMachine: logIndex=3 is != 1 + indexOfLastEntry=3",
	)

	// Multi append
	entries := []LogEntry{
		{TermNo: 1, Command: DummyCommand(104)},
		{TermNo: 1, Command: DummyCommand(105)},
	}
	err = sm.SetEntriesAfterIndex(3, entries)
	if err != nil {
		t.Fatal(err)
	}
	if !dsm.AppliedCommandsEqual(101, 102, 103, 104, 105) {
		t.Fatal(dsm)
	}

	// Rewinding multi append
	entries = []LogEntry{
		{TermNo: 2, Command: DummyCommand(203)},
		{TermNo: 2, Command: DummyCommand(204)},
	}
	err = sm.SetEntriesAfterIndex(2, entries)
	if err != nil {
		t.Fatal(err)
	}
	if !dsm.AppliedCommandsEqual(101, 102, 203, 204) {
		t.Fatal(dsm)
	}

	// Commit
	if sm.GetLastApplied() != 0 {
		t.Fatal(sm.GetLastApplied())
	}
	sm.CommitIndexChanged(3)
	if sm.GetLastApplied() != 3 { // XXX: assumes instant commit
		t.Fatal(sm.GetLastApplied())
	}

	// Attempt to rewind beyond lastApplied
	entries = []LogEntry{
		{TermNo: 2, Command: DummyCommand(303)},
		{TermNo: 2, Command: DummyCommand(304)},
	}

	err = sm.SetEntriesAfterIndex(2, entries)
	if err.Error() != "DummyStateMachine: logIndex=2 is < current commitIndex=3" {
		t.Fatal(dsm)
	}
	if !dsm.AppliedCommandsEqual(101, 102, 203, 204) {
		t.Fatal(dsm)
	}
}
