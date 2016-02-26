package raft

import (
	"reflect"
	"testing"
)

func TestLeaderVolatileState(t *testing.T) {
	ci, err := NewClusterInfo([]ServerId{"s1", "s2", "s3"}, "s3")
	if err != nil {
		t.Fatal(err)
	}

	lvs := newLeaderVolatileState(ci, 42)

	// Initial state
	// #5.3-p8s4: When a leader first comes to power, it initializes
	// all nextIndex values to the index just after the last one in
	// its log (11 in Figure 7).
	expectedNextIndex := map[ServerId]LogIndex{"s1": 43, "s2": 43}
	if !reflect.DeepEqual(lvs.nextIndex, expectedNextIndex) {
		t.Fatal(lvs.nextIndex)
	}
	expectedMatchIndex := map[ServerId]LogIndex{"s1": 0, "s2": 0}
	if !reflect.DeepEqual(lvs.matchIndex, expectedMatchIndex) {
		t.Fatal(lvs.matchIndex)
	}

	// getNextIndex
	if lvs.getNextIndex("s2") != 43 {
		t.Fatal()
	}
	test_ExpectPanic(
		t,
		func() {
			lvs.getNextIndex("s5")
		},
		"leaderVolatileState.getNextIndex(): unknown peer: s5",
	)

	// decrementNextIndex
	lvs.decrementNextIndex("s2")
	test_ExpectPanic(
		t,
		func() {
			lvs.decrementNextIndex("s3")
		},
		"leaderVolatileState.decrementNextIndex(): unknown peer: s3",
	)
	expectedNextIndex = map[ServerId]LogIndex{"s1": 43, "s2": 42}
	if !reflect.DeepEqual(lvs.nextIndex, expectedNextIndex) {
		t.Fatal(lvs.nextIndex)
	}
	if lvs.getNextIndex("s2") != 42 {
		t.Fatal()
	}
	lvs.nextIndex["s1"] = 1
	test_ExpectPanic(
		t,
		func() {
			lvs.decrementNextIndex("s1")
		},
		"leaderVolatileState.decrementNextIndex(): nextIndex <=1 for peer: s1",
	)

	// setMatchIndexAndNextIndex
	lvs.setMatchIndexAndNextIndex("s2", 24)
	expectedNextIndex = map[ServerId]LogIndex{"s1": 1, "s2": 25}
	if !reflect.DeepEqual(lvs.nextIndex, expectedNextIndex) {
		t.Fatal(lvs.nextIndex)
	}
	expectedMatchIndex = map[ServerId]LogIndex{"s1": 0, "s2": 24}
	if !reflect.DeepEqual(lvs.matchIndex, expectedMatchIndex) {
		t.Fatal(lvs.matchIndex)
	}
	lvs.setMatchIndexAndNextIndex("s2", 0)
	expectedNextIndex = map[ServerId]LogIndex{"s1": 1, "s2": 1}
	if !reflect.DeepEqual(lvs.nextIndex, expectedNextIndex) {
		t.Fatal(lvs.nextIndex)
	}
	expectedMatchIndex = map[ServerId]LogIndex{"s1": 0, "s2": 0}
	if !reflect.DeepEqual(lvs.matchIndex, expectedMatchIndex) {
		t.Fatal(lvs.matchIndex)
	}
}

// #RFS-L4: If there exists an N such that N > commitIndex, a majority
// of matchIndex[i] >= N, and log[N].term == currentTerm:
// set commitIndex = N (#5.3, #5.4)
func TestFindNewerCommitIndex_Figure8_CaseA(t *testing.T) {
	ci, err := NewClusterInfo([]ServerId{"s1", "s2", "s3", "s4", "s5"}, "s1")
	if err != nil {
		t.Fatal(err)
	}

	// Figure 8, case (a)
	terms := []TermNo{1, 2} // leader line for the case
	imle := newIMLEWithDummyCommands(terms, testMaxEntriesPerAppendEntry)
	lvs := newLeaderVolatileState(ci, LogIndex(len(terms)))

	// With matchIndex stuck at 0, there is no solution for any currentTerm
	if findNewerCommitIndex(ci, lvs, imle, 1, 0) != 0 {
		t.Fatal()
	}
	if findNewerCommitIndex(ci, lvs, imle, 2, 0) != 0 {
		t.Fatal()
	}
	if findNewerCommitIndex(ci, lvs, imle, 3, 0) != 0 {
		t.Fatal()
	}

	// match peers for Figure 8, case (a)
	lvs.setMatchIndexAndNextIndex("s2", 2)
	lvs.setMatchIndexAndNextIndex("s3", 1)
	lvs.setMatchIndexAndNextIndex("s4", 1)
	lvs.setMatchIndexAndNextIndex("s5", 1)

	// While we cannot be at currentTerm=1, it has a solution
	if findNewerCommitIndex(ci, lvs, imle, 1, 0) != 1 {
		t.Fatal()
	}
	if findNewerCommitIndex(ci, lvs, imle, 1, 1) != 0 {
		t.Fatal()
	}
	// No solution for currentTerm >= 2
	if findNewerCommitIndex(ci, lvs, imle, 2, 0) != 0 {
		t.Fatal()
	}
	if findNewerCommitIndex(ci, lvs, imle, 3, 0) != 0 {
		t.Fatal()
	}
}

func TestFindNewerCommitIndex_Figure8_CaseCAndE(t *testing.T) {
	ci, err := NewClusterInfo([]ServerId{"s1", "s2", "s3", "s4", "s5"}, "s1")
	if err != nil {
		t.Fatal(err)
	}

	// Figure 8, case (c)
	terms := []TermNo{1, 2, 4} // leader line for the case
	imle := newIMLEWithDummyCommands(terms, testMaxEntriesPerAppendEntry)
	lvs := newLeaderVolatileState(ci, LogIndex(len(terms)))

	// With matchIndex stuck at 0, there is no solution for any currentTerm
	if findNewerCommitIndex(ci, lvs, imle, 1, 0) != 0 {
		t.Fatal()
	}
	if findNewerCommitIndex(ci, lvs, imle, 2, 0) != 0 {
		t.Fatal()
	}
	if findNewerCommitIndex(ci, lvs, imle, 3, 0) != 0 {
		t.Fatal()
	}

	// match peers for Figure 8, case (c)
	lvs.setMatchIndexAndNextIndex("s2", 2)
	lvs.setMatchIndexAndNextIndex("s3", 2)
	lvs.setMatchIndexAndNextIndex("s4", 1)
	lvs.setMatchIndexAndNextIndex("s5", 1)

	// While we cannot be at currentTerm=1, it has solutions
	if findNewerCommitIndex(ci, lvs, imle, 1, 0) != 1 {
		t.Fatal()
	}
	if findNewerCommitIndex(ci, lvs, imle, 1, 1) != 0 {
		t.Fatal()
	}
	// While we cannot be at currentTerm=2, it has solutions
	if findNewerCommitIndex(ci, lvs, imle, 2, 0) != 2 {
		t.Fatal()
	}
	if findNewerCommitIndex(ci, lvs, imle, 2, 1) != 2 {
		t.Fatal()
	}
	// While we cannot be at currentTerm=3, it has no solution
	if findNewerCommitIndex(ci, lvs, imle, 3, 0) != 0 {
		t.Fatal()
	}
	if findNewerCommitIndex(ci, lvs, imle, 3, 1) != 0 {
		t.Fatal()
	}
	if findNewerCommitIndex(ci, lvs, imle, 3, 2) != 0 {
		t.Fatal()
	}
	// No solution for currentTerm >= 4
	if findNewerCommitIndex(ci, lvs, imle, 4, 0) != 0 {
		t.Fatal()
	}
	if findNewerCommitIndex(ci, lvs, imle, 4, 1) != 0 {
		t.Fatal()
	}
	if findNewerCommitIndex(ci, lvs, imle, 4, 2) != 0 {
		t.Fatal()
	}

	// match peers for Figure 8, case (e)
	lvs.setMatchIndexAndNextIndex("s2", 3)
	lvs.setMatchIndexAndNextIndex("s3", 3)
	lvs.setMatchIndexAndNextIndex("s4", 1)
	lvs.setMatchIndexAndNextIndex("s5", 1)

	// Now currentTerm = 4 has a solution
	if findNewerCommitIndex(ci, lvs, imle, 4, 0) != 3 {
		t.Fatal()
	}
	if findNewerCommitIndex(ci, lvs, imle, 4, 1) != 3 {
		t.Fatal()
	}
	if findNewerCommitIndex(ci, lvs, imle, 4, 2) != 3 {
		t.Fatal()
	}
	if findNewerCommitIndex(ci, lvs, imle, 4, 3) != 0 {
		t.Fatal()
	}
}

func TestFindNewerCommitIndex_Figure8_CaseEextended(t *testing.T) {
	ci, err := NewClusterInfo([]ServerId{"s1", "s2", "s3", "s4", "s5"}, "s1")
	if err != nil {
		t.Fatal(err)
	}

	// Figure 8, case (e) extended with extra term 4 entry at index 4
	terms := []TermNo{1, 2, 4, 4} // leader line for the case
	imle := newIMLEWithDummyCommands(terms, testMaxEntriesPerAppendEntry)
	lvs := newLeaderVolatileState(ci, LogIndex(len(terms)))

	// match peers
	lvs.setMatchIndexAndNextIndex("s2", 4)
	lvs.setMatchIndexAndNextIndex("s3", 4)
	lvs.setMatchIndexAndNextIndex("s4", 1)
	lvs.setMatchIndexAndNextIndex("s5", 1)

	// currentTerm = 4 has a solution
	// the test here captures the fact that first match is returned
	if findNewerCommitIndex(ci, lvs, imle, 4, 0) != 3 {
		t.Fatal()
	}
	if findNewerCommitIndex(ci, lvs, imle, 4, 1) != 3 {
		t.Fatal()
	}
	if findNewerCommitIndex(ci, lvs, imle, 4, 2) != 3 {
		t.Fatal()
	}
	// the next match is returned only after we cross the previous match
	if findNewerCommitIndex(ci, lvs, imle, 4, 3) != 4 {
		t.Fatal()
	}

}
