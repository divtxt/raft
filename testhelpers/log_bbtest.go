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
//
// To use the variant of this test with a compacted log (iofeIsFive=true), discard
// log entries before log index 5.
//
func BlackboxTest_Log(t *testing.T, log Log, iofeIsFive bool) {
	// Initial data tests
	iofe, err := log.GetIndexOfFirstEntry()
	if err != nil {
		t.Fatal(err)
	}
	if iofeIsFive {
		if iofe != 5 {
			t.Fatal(iofe)
		}
	} else {
		if iofe != 1 {
			t.Fatal(iofe)
		}
	}
	iole, err := log.GetIndexOfLastEntry()
	if err != nil {
		t.Fatal()
	}
	if iole != 10 {
		t.Fatal(iole)
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
	if iofeIsFive {
		entry, err := log.GetEntryAtIndex(4)
		if err != ErrIndexBeforeFirstEntry {
			t.Fatal(entry, err)
		}
	}
	entry, err := log.GetEntryAtIndex(5)
	if err != nil {
		t.Fatal(err)
	}
	if !TestCommandEquals(entry.Command, "c5") {
		t.Fatal(entry)
	}

	var logEntries []LogEntry

	// set test - invalid index
	if iofeIsFive {
		err = log.SetEntriesAfterIndex(3, logEntries)
		if err != ErrIndexBeforeFirstEntry {
			t.Fatal(err)
		}
	}
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
	err = log.SetEntriesAfterIndex(5, logEntries)
	if err != nil {
		t.Fatal(err)
	}
	iole, err = log.GetIndexOfLastEntry()
	if iole != 5 || err != nil {
		t.Fatal(iole, err)
	}
	le = TestHelper_GetLogEntryAtIndex(log, 5)
	if !reflect.DeepEqual(le, LogEntry{4, Command("c5")}) {
		t.Fatal(le)
	}
	if iofeIsFive {
		entry, err = log.GetEntryAtIndex(3)
		if err != ErrIndexBeforeFirstEntry {
			t.Fatal(entry, err)
		}
	}
}

// Test Helper
func TestHelper_GetLogEntryAtIndex(log internal.LogReadOnly, li LogIndex) LogEntry {
	if li == 0 {
		panic("oops!")
	}
	entry, err := log.GetEntryAtIndex(li)
	if err != nil {
		panic(err)
	}
	return entry
}
