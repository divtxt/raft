package lasm

import (
	"bytes"
	. "github.com/divtxt/raft"
	"reflect"
	"testing"
)

// Log with 10 entries with terms as shown in Figure 7, leader line
func BlackboxTest_MakeFigure7LeaderLineTerms() []TermNo {
	return []TermNo{1, 1, 1, 4, 4, 5, 5, 6, 6, 6}
}

// Helper
func TestCommandEquals(c Command, s string) bool {
	return bytes.Equal(c, Command(s))
}

// Blackbox test.
// Send a Log with 10 entries with terms as shown in Figure 7, leader line.
// No commands should have been applied yet.
func BlackboxTest_LogAndStateMachine(
	t *testing.T,
	lasm LogAndStateMachine,
	appendEntryOkValue interface{},
) {
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
	le := TestHelper_GetLogEntryAtIndex(lasm, 10)
	if le.TermNo != 6 {
		t.Fatal(le.TermNo)
	}
	if !TestCommandEquals(le.Command, "c10") {
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
	le = TestHelper_GetLogEntryAtIndex(lasm, 12)
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
	le = TestHelper_GetLogEntryAtIndex(lasm, 12)
	if !reflect.DeepEqual(le, LogEntry{9, Command("c12")}) {
		t.Fatal(le)
	}

	// append test
	result, err := lasm.AppendEntry(8, "c14")
	if err != nil {
		t.Fatal(err)
	}
	if result != appendEntryOkValue {
		t.Fatal(result)
	}
	if !reflect.DeepEqual(result, appendEntryOkValue) {
		t.Fatal(result)
	}
	iole, err = lasm.GetIndexOfLastEntry()
	if err != nil {
		t.Fatal()
	}
	if iole != 14 {
		t.Fatal()
	}
	le = TestHelper_GetLogEntryAtIndex(lasm, 14)
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
	le = TestHelper_GetLogEntryAtIndex(lasm, 3)
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
func TestHelper_GetLogEntryAtIndex(lasm LogAndStateMachine, li LogIndex) LogEntry {
	if li == 0 {
		panic("oops!")
	}
	entries, err := lasm.GetEntriesAfterIndex(li - 1)
	if err != nil {
		panic(err)
	}
	return entries[0]
}
