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

	// append test
	if log.appendEntriesAfterIndex(nil, 11) == nil {
		t.Fatal()
	}
	if log.appendEntriesAfterIndex(nil, 9) == nil {
		t.Fatal()
	}
	logEntries := []LogEntry{LogEntry{7, "c11"}, LogEntry{8, "c12"}}
	if e := log.appendEntriesAfterIndex(logEntries, 10); e != nil {
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

	// delete test
	log.deleteFromIndexToEnd(4)
	if i := log.getIndexOfLastEntry(); i != 3 {
		t.Fatal(i)
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

func (imle *inMemoryLog) appendEntriesAfterIndex(entries []LogEntry, li LogIndex) error {
	iole := imle.getIndexOfLastEntry()
	if iole != li {
		return fmt.Errorf("inMemoryLog: appendEntriesAfterIndex(..., %d) but iole=%d", iole)
	}
	imle.entries = append(imle.entries, entries...)
	return nil
}

func (imle *inMemoryLog) deleteFromIndexToEnd(li LogIndex) {
	imle.entries = imle.entries[:li-1]
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
