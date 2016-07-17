package lasm_test

import (
	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/lasm"
	"reflect"
	"testing"
)

// Run the blackbox test on inMemoryLog
func TestInMemoryLogEntries(t *testing.T) {
	// Log with 10 entries with terms as shown in Figure 7, leader line
	terms := lasm.BlackboxTest_MakeFigure7LeaderLineTerms()
	imle := lasm.NewDummyInMemoryLasmWithDummyCommands(terms, 3)
	lasm.BlackboxTest_LogAndStateMachine(t, imle, lasm.DummyInMemoryLasm_AppendEntry_Ok)
}

// Tests for inMemoryLog's GetEntriesAfterIndex implementation
func TestIMLE_GetEntriesAfterIndex(t *testing.T) {
	// Log with 10 entries with terms as shown in Figure 7, leader line
	terms := lasm.BlackboxTest_MakeFigure7LeaderLineTerms()
	imle := lasm.NewDummyInMemoryLasmWithDummyCommands(terms, 3)

	// none
	actualEntries, err := imle.GetEntriesAfterIndex(10)
	if err != nil {
		t.Fatal()
	}
	expectedEntries := []LogEntry{}
	if !reflect.DeepEqual(actualEntries, expectedEntries) {
		t.Fatal(actualEntries)
	}

	// one
	actualEntries, err = imle.GetEntriesAfterIndex(9)
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
	actualEntries, err = imle.GetEntriesAfterIndex(7)
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
	actualEntries, err = imle.GetEntriesAfterIndex(2)
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
	actualEntries, err = imle.GetEntriesAfterIndex(0)
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
	imle.MaxEntriesPerAppendEntry = 2
	actualEntries, err = imle.GetEntriesAfterIndex(2)
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
	actualEntries, err = imle.GetEntriesAfterIndex(11)
	if err.Error() != "afterLogIndex=11 is > iole=10" {
		t.Fatal(err)
	}
}
