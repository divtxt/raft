package raft

import (
	"fmt"
	"strconv"
	"testing"
)

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
	if le.TermNo != 8 {
		t.Fatal(le.TermNo)
	}
	if le.Command != "c12" {
		t.Fatal(le.Command)
	}

	// set test - partial replacing
	logEntries = []LogEntry{LogEntry{7, "c11"}, LogEntry{9, "c12'"}, LogEntry{9, "c13'"}}
	if e := log.setEntriesAfterIndex(10, logEntries); e != nil {
		t.Fatal(e)
	}
	if log.getIndexOfLastEntry() != 13 {
		t.Fatal()
	}
	le = log.getLogEntryAtIndex(12)
	if le.TermNo != 9 {
		t.Fatal(le.TermNo)
	}
	if le.Command != "c12'" {
		t.Fatal(le.Command)
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

func makeIMLEWithDummyCommands(logTerms []TermNo) *inMemoryLog {
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
	terms := []TermNo{1, 1, 1, 4, 4, 5, 5, 6, 6, 6}
	imle := makeIMLEWithDummyCommands(terms)
	LogBlackboxTest(t, imle)
}
