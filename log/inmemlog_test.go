package log

import (
	"reflect"
	"testing"

	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/testhelpers"
)

// Test InMemoryLog using the Log Blackbox test.
func TestInMemoryLog_BlackboxTest(t *testing.T) {
	inmem_log := TestUtil_NewInMemoryLog_WithFigure7LeaderLine(3)

	testhelpers.BlackboxTest_Log(t, inmem_log)
}

// Tests for InMemoryLog's GetEntriesAfterIndex implementation
func TestInMemoryLog_GetEntriesAfterIndex(t *testing.T) {
	// Log with 10 entries with terms as shown in Figure 7, leader line
	iml := TestUtil_NewInMemoryLog_WithFigure7LeaderLine(3)

	// none
	actualEntries, err := iml.GetEntriesAfterIndex(10)
	if err != nil {
		t.Fatal()
	}
	expectedEntries := []LogEntry{}
	if !reflect.DeepEqual(actualEntries, expectedEntries) {
		t.Fatal(actualEntries)
	}

	// one
	actualEntries, err = iml.GetEntriesAfterIndex(9)
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
	actualEntries, err = iml.GetEntriesAfterIndex(7)
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
	actualEntries, err = iml.GetEntriesAfterIndex(2)
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
	actualEntries, err = iml.GetEntriesAfterIndex(0)
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

	// index more than last log entry
	actualEntries, err = iml.GetEntriesAfterIndex(11)
	if err.Error() != "afterLogIndex=11 is > iole=10" {
		t.Fatal(err)
	}
}

// Tests for InMemoryLog's maxEntries policy implementation
func TestInMemoryLog_AlternateMaxEntries(t *testing.T) {
	// Log with 10 entries with terms as shown in Figure 7, leader line
	iml := TestUtil_NewInMemoryLog_WithFigure7LeaderLine(2)

	// max
	actualEntries, err := iml.GetEntriesAfterIndex(2)
	if err != nil {
		t.Fatal()
	}
	expectedEntries := []LogEntry{
		{1, Command("c3")},
		{4, Command("c4")},
	}
	if !reflect.DeepEqual(actualEntries, expectedEntries) {
		t.Fatal(actualEntries)
	}
}
