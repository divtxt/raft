package log

import (
	"testing"

	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/testhelpers"
)

// Test InMemoryLog using the Log Blackbox test.
func TestInMemoryLog_BlackboxTest(t *testing.T) {
	inmem_log, err := TestUtil_NewInMemoryLog_WithFigure7LeaderLine()
	if err != nil {
		t.Fatal(err)
	}

	testhelpers.BlackboxTest_Log(t, inmem_log, false)
}

// Test InMemoryLog compacted log using the Log Blackbox test.
func TestInMemoryLog_BlackboxTestWithCompaction(t *testing.T) {
	inmem_log, err := TestUtil_NewInMemoryLog_WithFigure7LeaderLine()
	if err != nil {
		t.Fatal(err)
	}

	err = inmem_log.DiscardEntriesBeforeIndex(5)
	if err != nil {
		t.Fatal(err)
	}

	testhelpers.BlackboxTest_Log(t, inmem_log, true)
}

// Tests for InMemoryLog's GetEntryAtIndex implementation
func TestInMemoryLog_GetEntryAtIndex(t *testing.T) {
	// Log with 10 entries with terms as shown in Figure 7, leader line
	iml, err := TestUtil_NewInMemoryLog_WithFigure7LeaderLine()
	if err != nil {
		t.Fatal(err)
	}

	// index of 0
	actualEntry, err := iml.GetEntryAtIndex(0)
	if err != ErrIndexBeforeFirstEntry {
		t.Fatal(err)
	}

	// index = 1
	actualEntry, err = iml.GetEntryAtIndex(1)
	if err != nil {
		t.Fatal()
	}
	expectedEntry := LogEntry{1, Command("c1")}
	if !actualEntry.Equals(expectedEntry) {
		t.Fatal(actualEntry)
	}

	// index < iole
	actualEntry, err = iml.GetEntryAtIndex(8)
	if err != nil {
		t.Fatal()
	}
	expectedEntry = LogEntry{6, Command("c8")}
	if !actualEntry.Equals(expectedEntry) {
		t.Fatal(actualEntry)
	}

	// index = iole
	actualEntry, err = iml.GetEntryAtIndex(10)
	if err != nil {
		t.Fatal()
	}
	expectedEntry = LogEntry{6, Command("c10")}
	if !actualEntry.Equals(expectedEntry) {
		t.Fatal(actualEntry)
	}

	// index > iole
	actualEntry, err = iml.GetEntryAtIndex(11)
	if err != ErrIndexAfterLastEntry {
		t.Fatal()
	}

	// log compaction
	iofe, err := iml.GetIndexOfFirstEntry()
	if iofe != 1 {
		t.Fatal(iofe)
	}
	if err != nil {
		t.Fatal(err)
	}
	err = iml.DiscardEntriesBeforeIndex(5)
	if err != nil {
		t.Fatal(err)
	}
	iofe, err = iml.GetIndexOfFirstEntry()
	if iofe != 5 || err != nil {
		t.Fatal(iofe, err)
	}

	// index < iofe
	actualEntry, err = iml.GetEntryAtIndex(3)
	if err != ErrIndexBeforeFirstEntry {
		t.Fatal(err)
	}

	// index = iofe
	actualEntry, err = iml.GetEntryAtIndex(5)
	if err != nil {
		t.Fatal()
	}
	expectedEntry = LogEntry{4, Command("c5")}
	if !actualEntry.Equals(expectedEntry) {
		t.Fatal(actualEntry)
	}

	// Extra test for DiscardEntriesBeforeIndex
	err = iml.DiscardEntriesBeforeIndex(4)
	if err != ErrIndexBeforeFirstEntry {
		t.Fatal(err)
	}
}
