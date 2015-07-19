package raft

import (
	"testing"
)

// Blackbox test
// Send a Log with 10 entries with terms as shown in Figure 7, leader line
func LogBlackboxTest(t *testing.T, le Log) {
	// Initial data tests
	if le.getIndexOfLastEntry() != 10 {
		t.Fatal()
	}
	if le.getTermAtIndex(10) != 6 {
		t.Fatal()
	}

	// delete test
	le.deleteFromIndexToEnd(4)
	if i := le.getIndexOfLastEntry(); i != 3 {
		t.Fatal(i)
	}
}

// In-memory implementation of LogEntries - meant only for tests
type inMemoryLog struct {
	terms []TermNo
}

func (imle *inMemoryLog) getIndexOfLastEntry() LogIndex {
	return LogIndex(len(imle.terms))
}

func (imle *inMemoryLog) getTermAtIndex(li LogIndex) TermNo {
	return imle.terms[li-1]
}

func (imle *inMemoryLog) deleteFromIndexToEnd(li LogIndex) {
	imle.terms = imle.terms[:li-1]
}

// Run the blackbox test on inMemoryLog
func TestInMemoryLogEntries(t *testing.T) {
	imle := new(inMemoryLog)
	imle.terms = []TermNo{1, 1, 1, 4, 4, 5, 5, 6, 6, 6}
	LogBlackboxTest(t, imle)
}
