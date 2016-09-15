package log

import (
	. "github.com/divtxt/raft"
	"reflect"
	"testing"
)

// Test InMemoryLog using the Log Blackbox test.
func TestInMemoryLog_BlackboxTest(t *testing.T) {
	inmem_log := TestUtil_NewInMemoryLog_WithFigure7LeaderLine()

	BlackboxTest_Log(t, inmem_log)
}

// Tests for InMemoryLog's GetEntriesAfterIndex implementation
func TestInMemoryLog_GetEntriesAfterIndex(t *testing.T) {
	// Log with 10 entries with terms as shown in Figure 7, leader line
	iml := TestUtil_NewInMemoryLog_WithFigure7LeaderLine()
	var maxEntries uint64 = 3

	// none
	actualEntries, err := iml.GetEntriesAfterIndex(10, maxEntries)
	if err != nil {
		t.Fatal()
	}
	expectedEntries := []LogEntry{}
	if !reflect.DeepEqual(actualEntries, expectedEntries) {
		t.Fatal(actualEntries)
	}

	// one
	actualEntries, err = iml.GetEntriesAfterIndex(9, maxEntries)
	if err != nil {
		t.Fatal()
	}
	expectedEntries = []LogEntry{
		{6, Command("c10")},
	}
	if !reflect.DeepEqual(actualEntries, expectedEntries) {
		t.Fatal(actualEntries)
	}

	// multiple
	actualEntries, err = iml.GetEntriesAfterIndex(7, maxEntries)
	if err != nil {
		t.Fatal()
	}
	expectedEntries = []LogEntry{
		{6, Command("c8")},
		{6, Command("c9")},
		{6, Command("c10")},
	}
	if !reflect.DeepEqual(actualEntries, expectedEntries) {
		t.Fatal(actualEntries)
	}

	// max
	actualEntries, err = iml.GetEntriesAfterIndex(2, maxEntries)
	if err != nil {
		t.Fatal()
	}
	expectedEntries = []LogEntry{
		{1, Command("c3")},
		{4, Command("c4")},
		{4, Command("c5")},
	}
	if !reflect.DeepEqual(actualEntries, expectedEntries) {
		t.Fatal(actualEntries)
	}

	// index of 0
	actualEntries, err = iml.GetEntriesAfterIndex(0, maxEntries)
	if err != nil {
		t.Fatal()
	}
	expectedEntries = []LogEntry{
		{1, Command("c1")},
		{1, Command("c2")},
		{1, Command("c3")},
	}
	if !reflect.DeepEqual(actualEntries, expectedEntries) {
		t.Fatal(actualEntries)
	}

	// max alternate value
	actualEntries, err = iml.GetEntriesAfterIndex(2, 2)
	if err != nil {
		t.Fatal()
	}
	expectedEntries = []LogEntry{
		{1, Command("c3")},
		{4, Command("c4")},
	}
	if !reflect.DeepEqual(actualEntries, expectedEntries) {
		t.Fatal(actualEntries)
	}

	// index more than last log entry
	actualEntries, err = iml.GetEntriesAfterIndex(11, maxEntries)
	if err.Error() != "afterLogIndex=11 is > iole=10" {
		t.Fatal(err)
	}
}
