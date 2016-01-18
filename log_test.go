package raft

import (
	"bytes"
	"fmt"
	"reflect"
	"strconv"
	"testing"
)

// Log with 10 entries with terms as shown in Figure 7, leader line
func makeLogTerms_Figure7LeaderLine() []TermNo {
	return []TermNo{1, 1, 1, 4, 4, 5, 5, 6, 6, 6}
}

// Helper
func testCommandEquals(c Command, s string) bool {
	return bytes.Equal(c, Command(s))
}

// Blackbox test
// Send a Log with 10 entries with terms as shown in Figure 7, leader line.
// No commands should have been applied yet.
func PartialTest_Log_BlackboxTest(t *testing.T, log Log) {
	// Initial data tests
	if log.GetIndexOfLastEntry() != 10 {
		t.Fatal()
	}
	if log.GetTermAtIndex(10) != 6 {
		t.Fatal()
	}

	// get entry test
	le := log.GetLogEntryAtIndex(10)
	if le.TermNo != 6 {
		t.Fatal(le.TermNo)
	}
	if !testCommandEquals(le.Command, "c10") {
		t.Fatal(le.Command)
	}

	var logEntries []LogEntry

	// set test - invalid index
	logEntries = []LogEntry{{8, Command("c12")}}
	test_ExpectPanicAnyRecover(
		t,
		func() {
			log.SetEntriesAfterIndex(11, logEntries)
		},
	)

	// set test - no replacing
	logEntries = []LogEntry{{7, Command("c11")}, {8, Command("c12")}}
	log.SetEntriesAfterIndex(10, logEntries)
	if log.GetIndexOfLastEntry() != 12 {
		t.Fatal()
	}
	le = log.GetLogEntryAtIndex(12)
	if !reflect.DeepEqual(le, LogEntry{8, Command("c12")}) {
		t.Fatal(le)
	}

	// set test - partial replacing
	logEntries = []LogEntry{{7, Command("c11")}, {9, Command("c12")}, {9, Command("c13'")}}
	log.SetEntriesAfterIndex(10, logEntries)
	if log.GetIndexOfLastEntry() != 13 {
		t.Fatal()
	}
	le = log.GetLogEntryAtIndex(12)
	if !reflect.DeepEqual(le, LogEntry{9, Command("c12")}) {
		t.Fatal(le)
	}

	// commitIndex tests
	log.CommitIndexChanged(1)
	log.CommitIndexChanged(3)
	test_ExpectPanicAnyRecover(
		t,
		func() {
			log.CommitIndexChanged(2)
		},
	)

	// set test - no new entries with empty slice
	logEntries = []LogEntry{}
	log.SetEntriesAfterIndex(3, logEntries)
	if log.GetIndexOfLastEntry() != 3 {
		t.Fatal()
	}
	le = log.GetLogEntryAtIndex(3)
	if !reflect.DeepEqual(le, LogEntry{1, Command("c3")}) {
		t.Fatal(le)
	}

	// commitIndex test - error to go past end of log
	test_ExpectPanicAnyRecover(
		t,
		func() {
			log.CommitIndexChanged(4)
		},
	)

	// set test - error to modify log before commitIndex
	test_ExpectPanicAnyRecover(
		t,
		func() {
			logEntries = []LogEntry{}
			log.SetEntriesAfterIndex(2, logEntries)
		},
	)
}

// In-memory implementation of LogEntries - meant only for tests
type inMemoryLog struct {
	entries                  []LogEntry
	_commitIndex             LogIndex
	maxEntriesPerAppendEntry uint64
}

func (imle *inMemoryLog) GetIndexOfLastEntry() LogIndex {
	return LogIndex(len(imle.entries))
}

func (imle *inMemoryLog) GetTermAtIndex(li LogIndex) TermNo {
	if li == 0 {
		panic("GetTermAtIndex(): li=0")
	}
	if li > LogIndex(len(imle.entries)) {
		panic(fmt.Sprintf("GetTermAtIndex(): li=%v > iole=%v", li, len(imle.entries)))
	}
	return imle.entries[li-1].TermNo
}

func (imle *inMemoryLog) GetLogEntryAtIndex(li LogIndex) LogEntry {
	return imle.entries[li-1]
}

func (imle *inMemoryLog) SetEntriesAfterIndex(li LogIndex, entries []LogEntry) {
	if li < imle._commitIndex {
		panic(fmt.Sprintf(
			"inMemoryLog: setEntriesAfterIndex(%d, ...) but commitIndex=%d",
			li,
			imle._commitIndex,
		))
	}
	iole := imle.GetIndexOfLastEntry()
	if iole < li {
		panic(fmt.Sprintf("inMemoryLog: setEntriesAfterIndex(%d, ...) but iole=%d", li, iole))
	}
	// delete entries after index
	if iole > li {
		imle.entries = imle.entries[:li]
	}
	// append entries
	imle.entries = append(imle.entries, entries...)
}

func (imle *inMemoryLog) GetEntriesAfterIndex(afterLogIndex LogIndex) []LogEntry {
	iole := imle.GetIndexOfLastEntry()

	if iole < afterLogIndex {
		panic(fmt.Sprintf(
			"afterLogIndex=%v is > iole=%v",
			afterLogIndex,
			iole,
		))
	}

	var numEntriesToGet uint64 = uint64(iole - afterLogIndex)

	// Short-circuit allocation for common case
	if numEntriesToGet == 0 {
		return []LogEntry{}
	}

	if numEntriesToGet > imle.maxEntriesPerAppendEntry {
		numEntriesToGet = imle.maxEntriesPerAppendEntry
	}

	logEntries := make([]LogEntry, numEntriesToGet)
	var i uint64 = 0
	nextIndexToGet := afterLogIndex + 1

	for i < numEntriesToGet {
		logEntries[i] = imle.GetLogEntryAtIndex(nextIndexToGet)
		i++
		nextIndexToGet++
	}

	return logEntries
}

func (imle *inMemoryLog) CommitIndexChanged(commitIndex LogIndex) {
	if commitIndex < imle._commitIndex {
		panic(fmt.Sprintf(
			"inMemoryLog: CommitIndexChanged(%d) is < current commitIndex=%d",
			commitIndex,
			imle._commitIndex,
		))
	}
	iole := imle.GetIndexOfLastEntry()
	if commitIndex > iole {
		panic(fmt.Sprintf(
			"inMemoryLog: CommitIndexChanged(%d) is > iole=%d",
			commitIndex,
			iole,
		))
	}
	imle._commitIndex = commitIndex
}

func newIMLEWithDummyCommands(
	logTerms []TermNo,
	maxEntriesPerAppendEntry uint64,
) *inMemoryLog {
	if maxEntriesPerAppendEntry <= 0 {
		panic("maxEntriesPerAppendEntry must be greater than zero")
	}
	entries := []LogEntry{}
	for i, term := range logTerms {
		entries = append(entries, LogEntry{term, Command("c" + strconv.Itoa(i+1))})
	}
	imle := &inMemoryLog{
		entries,
		0,
		maxEntriesPerAppendEntry,
	}
	return imle
}

// Run the blackbox test on inMemoryLog
func TestInMemoryLogEntries(t *testing.T) {
	// Log with 10 entries with terms as shown in Figure 7, leader line
	terms := makeLogTerms_Figure7LeaderLine()
	imle := newIMLEWithDummyCommands(terms, 3)
	PartialTest_Log_BlackboxTest(t, imle)
}

// Tests for inMemoryLog's GetEntriesAfterIndex implementation
func TestIMLE_GetEntriesAfterIndex(t *testing.T) {
	// Log with 10 entries with terms as shown in Figure 7, leader line
	terms := makeLogTerms_Figure7LeaderLine()
	imle := newIMLEWithDummyCommands(terms, 3)

	// none
	actualEntries := imle.GetEntriesAfterIndex(10)
	expectedEntries := []LogEntry{}
	if !reflect.DeepEqual(actualEntries, expectedEntries) {
		t.Fatal(actualEntries)
	}

	// one
	actualEntries = imle.GetEntriesAfterIndex(9)
	expectedEntries = []LogEntry{
		{6, Command("c10")},
	}
	if !reflect.DeepEqual(actualEntries, expectedEntries) {
		t.Fatal(actualEntries)
	}

	// multiple
	actualEntries = imle.GetEntriesAfterIndex(7)
	expectedEntries = []LogEntry{
		{6, Command("c8")},
		{6, Command("c9")},
		{6, Command("c10")},
	}
	if !reflect.DeepEqual(actualEntries, expectedEntries) {
		t.Fatal(actualEntries)
	}

	// max
	actualEntries = imle.GetEntriesAfterIndex(2)
	expectedEntries = []LogEntry{
		{1, Command("c3")},
		{4, Command("c4")},
		{4, Command("c5")},
	}
	if !reflect.DeepEqual(actualEntries, expectedEntries) {
		t.Fatal(actualEntries)
	}

	// index of 0
	actualEntries = imle.GetEntriesAfterIndex(0)
	expectedEntries = []LogEntry{
		{1, Command("c1")},
		{1, Command("c2")},
		{1, Command("c3")},
	}
	if !reflect.DeepEqual(actualEntries, expectedEntries) {
		t.Fatal(actualEntries)
	}

	// max alternate value
	imle.maxEntriesPerAppendEntry = 2
	actualEntries = imle.GetEntriesAfterIndex(2)
	expectedEntries = []LogEntry{
		{1, Command("c3")},
		{4, Command("c4")},
	}
	if !reflect.DeepEqual(actualEntries, expectedEntries) {
		t.Fatal(actualEntries)
	}

	// index more than last log entry
	test_ExpectPanic(
		t,
		func() {
			imle.GetEntriesAfterIndex(11)
		},
		"afterLogIndex=11 is > iole=10",
	)
}
