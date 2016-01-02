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
	entries      []LogEntry
	_commitIndex LogIndex
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

func newIMLEWithDummyCommands(logTerms []TermNo) *inMemoryLog {
	imle := new(inMemoryLog)
	entries := []LogEntry{}
	for i, term := range logTerms {
		entries = append(entries, LogEntry{term, Command("c" + strconv.Itoa(i+1))})
	}
	imle.entries = entries
	return imle
}

// Run the blackbox test on inMemoryLog
func TestInMemoryLogEntries(t *testing.T) {
	// Log with 10 entries with terms as shown in Figure 7, leader line
	terms := makeLogTerms_Figure7LeaderLine()
	imle := newIMLEWithDummyCommands(terms)
	PartialTest_Log_BlackboxTest(t, imle)
}
