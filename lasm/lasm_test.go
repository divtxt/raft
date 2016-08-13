package lasm_test

import (
	. "github.com/divtxt/raft"
	raft_lasm "github.com/divtxt/raft/lasm"
	raft_log "github.com/divtxt/raft/log"
	"reflect"
	"testing"
)

// Run the blackbox test on inMemoryLog
func TestLogAndStateMachineImpl_A(t *testing.T) {
	iml := raft_log.TestUtil_NewInMemoryLog_WithFigure7LeaderLine(10)
	dsm := raft_lasm.NewDummyStateMachine()
	lasm := raft_lasm.NewLogAndStateMachineImpl(iml, dsm)

	// Initial data tests
	iole, err := lasm.GetIndexOfLastEntry()
	if err != nil {
		t.Fatal()
	}
	if iole != 10 {
		t.Fatal()
	}
	term, err := lasm.GetTermAtIndex(10)
	if err != nil {
		t.Fatal(err)
	}
	if term != 6 {
		t.Fatal()
	}

	// get entries test
	le := Helper_GetLogEntryAtIndex(lasm, 10)
	if le.TermNo != 6 {
		t.Fatal(le.TermNo)
	}
	if !raft_lasm.DummyCommandEquals(le.Command, 10) {
		t.Fatal(le.Command)
	}

	var logEntries []LogEntry

	// set test - invalid index
	logEntries = []LogEntry{{8, Command("c12")}}
	err = lasm.SetEntriesAfterIndex(11, logEntries)
	if err == nil {
		t.Fatal()
	}

	// set test - no replacing
	logEntries = []LogEntry{{7, Command("c11")}, {8, Command("c12")}}
	err = lasm.SetEntriesAfterIndex(10, logEntries)
	if err != nil {
		t.Fatal()
	}
	iole, err = lasm.GetIndexOfLastEntry()
	if err != nil {
		t.Fatal()
	}
	if iole != 12 {
		t.Fatal()
	}
	le = Helper_GetLogEntryAtIndex(lasm, 12)
	if !reflect.DeepEqual(le, LogEntry{8, Command("c12")}) {
		t.Fatal(le)
	}

	// set test - partial replacing
	logEntries = []LogEntry{{7, Command("c11")}, {9, Command("c12")}, {9, Command("c13'")}}
	err = lasm.SetEntriesAfterIndex(10, logEntries)
	if err != nil {
		t.Fatal()
	}
	iole, err = lasm.GetIndexOfLastEntry()
	if err != nil {
		t.Fatal()
	}
	if iole != 13 {
		t.Fatal()
	}
	le = Helper_GetLogEntryAtIndex(lasm, 12)
	if !reflect.DeepEqual(le, LogEntry{9, Command("c12")}) {
		t.Fatal(le)
	}

	// append test
	reply, err := lasm.AppendEntry(8, raft_lasm.DummyCommand{14, false})
	if err != nil {
		t.Fatal(err)
	}
	if reply != raft_lasm.DummyCommand_Reply_Ok {
		t.Fatal(reply)
	}
	iole, err = lasm.GetIndexOfLastEntry()
	if err != nil {
		t.Fatal()
	}
	if iole != 14 {
		t.Fatal()
	}
	le = Helper_GetLogEntryAtIndex(lasm, 14)
	if !reflect.DeepEqual(le, LogEntry{8, Command("c14")}) {
		t.Fatal(le)
	}

	// append test - rejected entry
	reply, err = lasm.AppendEntry(8, raft_lasm.DummyCommand{15, true})
	if err != nil || reply != raft_lasm.DummyCommand_Reply_Reject {
		t.Fatal()
	}
	iole, err = lasm.GetIndexOfLastEntry()
	if err != nil || iole != 14 {
		t.Fatal()
	}
	le = Helper_GetLogEntryAtIndex(lasm, 14)
	if !reflect.DeepEqual(le, LogEntry{8, Command("c14")}) {
		t.Fatal(le)
	}

	// commitIndex tests
	err = lasm.CommitIndexChanged(1)
	if err != nil {
		t.Fatal(err)
	}
	err = lasm.CommitIndexChanged(3)
	if err != nil {
		t.Fatal(err)
	}
	err = lasm.CommitIndexChanged(2)
	if err == nil {
		t.Fatal()
	}

	// set test - no new entries with empty slice
	logEntries = []LogEntry{}
	err = lasm.SetEntriesAfterIndex(3, logEntries)
	if err != nil {
		t.Fatal()
	}
	iole, err = lasm.GetIndexOfLastEntry()
	if err != nil {
		t.Fatal()
	}
	if iole != 3 {
		t.Fatal()
	}
	le = Helper_GetLogEntryAtIndex(lasm, 3)
	if !reflect.DeepEqual(le, LogEntry{1, Command("c3")}) {
		t.Fatal(le)
	}

	// commitIndex test - error to go past end of log
	err = lasm.CommitIndexChanged(4)
	if err == nil {
		t.Fatal()
	}

	// set test - error to modify log before commitIndex
	err = lasm.SetEntriesAfterIndex(2, []LogEntry{})
	if err == nil {
		t.Fatal()
	}

}

// Helper
func Helper_GetLogEntryAtIndex(lasm LogAndStateMachine, li LogIndex) LogEntry {
	if li == 0 {
		panic("oops!")
	}
	entries, err := lasm.GetEntriesAfterIndex(li - 1)
	if err != nil {
		panic(err)
	}
	return entries[0]
}
