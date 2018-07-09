package testhelpers

import (
	"bytes"
	"reflect"
	"testing"

	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/internal"
)

// Helper
func TestCommandEquals(c Command, s string) bool {
	return bytes.Equal(c, Command(s))
}

// Blackbox test.
//
// Send a Log with 10 entries with terms as shown in Figure 7, leader line.
// Entries should be Command("c1"), Command("c2"), etc.
// GetEntriesAfterIndex() policy should not return more than 3 entries.
func BlackboxTest_Log(t *testing.T, log Log) {
	// Initial data tests
	iole, err := log.GetIndexOfLastEntry()
	if err != nil {
		t.Fatal()
	}
	if iole != 10 {
		t.Fatal()
	}
	term, err := log.GetTermAtIndex(10)
	if err != nil {
		t.Fatal(err)
	}
	if term != 6 {
		t.Fatal()
	}

	// get entries test
	le := TestHelper_GetLogEntryAtIndex(log, 10)
	if le.TermNo != 6 {
		t.Fatal(le.TermNo)
	}
	if !TestCommandEquals(le.Command, "c10") {
		t.Fatal(le.Command)
	}

	// get multiple entries
	entries, err := log.GetEntriesAfterIndex(4)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatal()
	}
	if !TestCommandEquals(entries[0].Command, "c5") {
		t.Fatal()
	}
	if !TestCommandEquals(entries[1].Command, "c6") {
		t.Fatal()
	}
	if !TestCommandEquals(entries[2].Command, "c7") {
		t.Fatal()
	}

	var logEntries []LogEntry

	// set test - invalid index
	logEntries = []LogEntry{{8, Command("c12")}}
	err = log.SetEntriesAfterIndex(11, logEntries)
	if err == nil {
		t.Fatal()
	}

	// set test - no replacing
	logEntries = []LogEntry{{7, Command("c11")}, {8, Command("c12")}}
	err = log.SetEntriesAfterIndex(10, logEntries)
	if err != nil {
		t.Fatal()
	}
	iole, err = log.GetIndexOfLastEntry()
	if err != nil {
		t.Fatal()
	}
	if iole != 12 {
		t.Fatal()
	}
	le = TestHelper_GetLogEntryAtIndex(log, 12)
	if !reflect.DeepEqual(le, LogEntry{8, Command("c12")}) {
		t.Fatal(le)
	}

	// set test - partial replacing
	logEntries = []LogEntry{{7, Command("c11")}, {9, Command("c12")}, {9, Command("c13'")}}
	err = log.SetEntriesAfterIndex(10, logEntries)
	if err != nil {
		t.Fatal()
	}
	iole, err = log.GetIndexOfLastEntry()
	if err != nil {
		t.Fatal()
	}
	if iole != 13 {
		t.Fatal()
	}
	le = TestHelper_GetLogEntryAtIndex(log, 12)
	if !reflect.DeepEqual(le, LogEntry{9, Command("c12")}) {
		t.Fatal(le)
	}

	// append test
	logEntry := LogEntry{8, Command("c14")}
	ioleAE, err := log.AppendEntry(logEntry)
	if err != nil {
		t.Fatal(err)
	}
	if ioleAE != 14 {
		t.Fatal(ioleAE)
	}
	iole, err = log.GetIndexOfLastEntry()
	if err != nil {
		t.Fatal()
	}
	if iole != 14 {
		t.Fatal()
	}
	le = TestHelper_GetLogEntryAtIndex(log, 14)
	if !reflect.DeepEqual(le, LogEntry{8, Command("c14")}) {
		t.Fatal(le)
	}

	// set test - no new entries with empty slice
	logEntries = []LogEntry{}
	err = log.SetEntriesAfterIndex(3, logEntries)
	if err != nil {
		t.Fatal()
	}
	iole, err = log.GetIndexOfLastEntry()
	if err != nil {
		t.Fatal()
	}
	if iole != 3 {
		t.Fatal()
	}
	le = TestHelper_GetLogEntryAtIndex(log, 3)
	if !reflect.DeepEqual(le, LogEntry{1, Command("c3")}) {
		t.Fatal(le)
	}
}

// Test Helper
func TestHelper_GetLogEntryAtIndex(log internal.LogReadOnly, li LogIndex) LogEntry {
	if li == 0 {
		panic("oops!")
	}
	entries, err := log.GetEntriesAfterIndex(li - 1)
	if err != nil {
		panic(err)
	}
	return entries[0]
}
