package raft

import (
	"fmt"
	"reflect"
	"strconv"
	"testing"
)

// Log with 10 entries with terms as shown in Figure 7, leader line
func testLogTerms_Figure7LeaderLine() []TermNo {
	return []TermNo{1, 1, 1, 4, 4, 5, 5, 6, 6, 6}
}

// Blackbox test
// Send a Log with 10 entries with terms as shown in Figure 7, leader line
func LogBlackboxTest(t *testing.T, log Log) {
	// Initial data tests
	if log.getIndexOfLastEntry() != 10 {
		t.Fatal()
	}
	if log.getTermAtIndex(10) != 6 {
		t.Fatal()
	}

	// get entry test
	le := log.getLogEntryAtIndex(10)
	if le.TermNo != 6 {
		t.Fatal(le.TermNo)
	}
	if le.Command != "c10" {
		t.Fatal(le.Command)
	}

	var logEntries []LogEntry

	// set test - invalid index
	logEntries = []LogEntry{LogEntry{8, "c12"}}
	if log.setEntriesAfterIndex(11, logEntries) == nil {
		t.Fatal()
	}

	// set test - no replacing
	logEntries = []LogEntry{LogEntry{7, "c11"}, LogEntry{8, "c12"}}
	if e := log.setEntriesAfterIndex(10, logEntries); e != nil {
		t.Fatal(e)
	}
	if log.getIndexOfLastEntry() != 12 {
		t.Fatal()
	}
	le = log.getLogEntryAtIndex(12)
	if !reflect.DeepEqual(le, LogEntry{8, "c12"}) {
		t.Fatal(le)
	}

	// set test - partial replacing
	logEntries = []LogEntry{LogEntry{7, "c11"}, LogEntry{9, "c12"}, LogEntry{9, "c13'"}}
	if e := log.setEntriesAfterIndex(10, logEntries); e != nil {
		t.Fatal(e)
	}
	if log.getIndexOfLastEntry() != 13 {
		t.Fatal()
	}
	le = log.getLogEntryAtIndex(12)
	if !reflect.DeepEqual(le, LogEntry{9, "c12"}) {
		t.Fatal(le)
	}

	// set test - no new entries with empty slice
	logEntries = []LogEntry{}
	if e := log.setEntriesAfterIndex(3, logEntries); e != nil {
		t.Fatal(e)
	}
	if log.getIndexOfLastEntry() != 3 {
		t.Fatal()
	}
	le = log.getLogEntryAtIndex(3)
	if !reflect.DeepEqual(le, LogEntry{1, "c3"}) {
		t.Fatal(le)
	}

	// set test - delete all entries; no new entries with nil
	if e := log.setEntriesAfterIndex(0, nil); e != nil {
		t.Fatal(e)
	}
	if log.getIndexOfLastEntry() != 0 {
		t.Fatal()
	}

}

// In-memory implementation of LogEntries - meant only for tests
type inMemoryLog struct {
	entries []LogEntry
}

func (imle *inMemoryLog) getIndexOfLastEntry() LogIndex {
	return LogIndex(len(imle.entries))
}

func (imle *inMemoryLog) getTermAtIndex(li LogIndex) TermNo {
	return imle.entries[li-1].TermNo
}

func (imle *inMemoryLog) getLogEntryAtIndex(li LogIndex) LogEntry {
	return imle.entries[li-1]
}

func (imle *inMemoryLog) setEntriesAfterIndex(li LogIndex, entries []LogEntry) error {
	iole := imle.getIndexOfLastEntry()
	if iole < li {
		return fmt.Errorf("inMemoryLog: setEntriesAfterIndex(%d, ...) but iole=%d", iole)
	}
	// delete entries after index
	if iole > li {
		imle.entries = imle.entries[:li]
	}
	// append entries
	imle.entries = append(imle.entries, entries...)
	return nil
}

func newIMLEWithDummyCommands(logTerms []TermNo) *inMemoryLog {
	imle := new(inMemoryLog)
	entries := []LogEntry{}
	for i, term := range logTerms {
		entries = append(entries, LogEntry{term, "c" + strconv.Itoa(i+1)})
	}
	imle.entries = entries
	return imle
}

// Run the blackbox test on inMemoryLog
func TestInMemoryLogEntries(t *testing.T) {
	// Log with 10 entries with terms as shown in Figure 7, leader line
	terms := testLogTerms_Figure7LeaderLine()
	imle := newIMLEWithDummyCommands(terms)
	LogBlackboxTest(t, imle)
}
