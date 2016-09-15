package consensus_state_test

import (
	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/config"
	consensus_state "github.com/divtxt/raft/consensus/state"
	"github.com/divtxt/raft/log"
	"reflect"
	"testing"
)

func TestLeaderVolatileState(t *testing.T) {
	ci, err := config.NewClusterInfo([]ServerId{"s1", "s2", "s3"}, "s3")
	if err != nil {
		t.Fatal(err)
	}

	lvs, err := consensus_state.NewLeaderVolatileState(ci, 42)
	if err != nil {
		t.Fatal(err)
	}

	// Initial state
	// #5.3-p8s4: When a leader first comes to power, it initializes
	// all nextIndex values to the index just after the last one in
	// its log (11 in Figure 7).
	expectedNextIndex := map[ServerId]LogIndex{"s1": 43, "s2": 43}
	if !reflect.DeepEqual(lvs.NextIndex, expectedNextIndex) {
		t.Fatal(lvs.NextIndex)
	}
	expectedMatchIndex := map[ServerId]LogIndex{"s1": 0, "s2": 0}
	if !reflect.DeepEqual(lvs.MatchIndex, expectedMatchIndex) {
		t.Fatal(lvs.MatchIndex)
	}

	// getNextIndex
	idx, err := lvs.GetNextIndex("s2")
	if err != nil {
		t.Fatal(err)
	}
	if idx != 43 {
		t.Fatal()
	}

	idx, err = lvs.GetNextIndex("s5")
	if err.Error() != "LeaderVolatileState.GetNextIndex(): unknown peer: s5" {
		t.Fatal(err)
	}

	// DecrementNextIndex
	err = lvs.DecrementNextIndex("s2")
	if err != nil {
		t.Fatal(err)
	}
	err = lvs.DecrementNextIndex("s3")
	if err.Error() != "LeaderVolatileState.DecrementNextIndex(): unknown peer: s3" {
		t.Fatal(err)
	}
	expectedNextIndex = map[ServerId]LogIndex{"s1": 43, "s2": 42}
	if !reflect.DeepEqual(lvs.NextIndex, expectedNextIndex) {
		t.Fatal(lvs.NextIndex)
	}
	idx, err = lvs.GetNextIndex("s2")
	if err != nil {
		t.Fatal(err)
	}
	if idx != 42 {
		t.Fatal()
	}

	lvs.NextIndex["s1"] = 1
	err = lvs.DecrementNextIndex("s1")
	if err.Error() != "LeaderVolatileState.DecrementNextIndex(): nextIndex <=1 for peer: s1" {
		t.Fatal(err)
	}

	// SetMatchIndexAndNextIndex
	err = lvs.SetMatchIndexAndNextIndex("s2", 24)
	if err != nil {
		t.Fatal(err)
	}
	expectedNextIndex = map[ServerId]LogIndex{"s1": 1, "s2": 25}
	if !reflect.DeepEqual(lvs.NextIndex, expectedNextIndex) {
		t.Fatal(lvs.NextIndex)
	}
	expectedMatchIndex = map[ServerId]LogIndex{"s1": 0, "s2": 24}
	if !reflect.DeepEqual(lvs.MatchIndex, expectedMatchIndex) {
		t.Fatal(lvs.MatchIndex)
	}
	err = lvs.SetMatchIndexAndNextIndex("s2", 0)
	if err != nil {
		t.Fatal(err)
	}
	expectedNextIndex = map[ServerId]LogIndex{"s1": 1, "s2": 1}
	if !reflect.DeepEqual(lvs.NextIndex, expectedNextIndex) {
		t.Fatal(lvs.NextIndex)
	}
	expectedMatchIndex = map[ServerId]LogIndex{"s1": 0, "s2": 0}
	if !reflect.DeepEqual(lvs.MatchIndex, expectedMatchIndex) {
		t.Fatal(lvs.MatchIndex)
	}
}

// #RFS-L4: If there exists an N such that N > commitIndex, a majority
// of matchIndex[i] >= N, and log[N].term == currentTerm:
// set commitIndex = N (#5.3, #5.4)
func TestFindNewerCommitIndex_Figure8_CaseA(t *testing.T) {
	ci, err := config.NewClusterInfo([]ServerId{"s1", "s2", "s3", "s4", "s5"}, "s1")
	if err != nil {
		t.Fatal(err)
	}

	// Figure 8, case (a)
	terms := []TermNo{1, 2} // leader line for the case
	imle := log.TestUtil_NewInMemoryLog_WithTerms(terms)
	lvs, err := consensus_state.NewLeaderVolatileState(ci, LogIndex(len(terms)))
	if err != nil {
		t.Fatal(err)
	}

	_findNewerCommitIndex := func(currentTerm TermNo, commitIndex LogIndex) LogIndex {
		nci, err := consensus_state.FindNewerCommitIndex(ci, lvs, imle, currentTerm, commitIndex)
		if err != nil {
			t.Fatal(err)
		}
		return nci
	}

	// With matchIndex stuck at 0, there is no solution for any currentTerm
	if _findNewerCommitIndex(1, 0) != 0 {
		t.Fatal()
	}
	if _findNewerCommitIndex(2, 0) != 0 {
		t.Fatal()
	}
	if _findNewerCommitIndex(3, 0) != 0 {
		t.Fatal()
	}

	// match peers for Figure 8, case (a)
	err = lvs.SetMatchIndexAndNextIndex("s2", 2)
	if err != nil {
		t.Fatal(err)
	}
	err = lvs.SetMatchIndexAndNextIndex("s3", 1)
	if err != nil {
		t.Fatal(err)
	}
	err = lvs.SetMatchIndexAndNextIndex("s4", 1)
	if err != nil {
		t.Fatal(err)
	}
	err = lvs.SetMatchIndexAndNextIndex("s5", 1)
	if err != nil {
		t.Fatal(err)
	}

	// While we cannot be at currentTerm=1, it has a solution
	if _findNewerCommitIndex(1, 0) != 1 {
		t.Fatal()
	}
	if _findNewerCommitIndex(1, 1) != 0 {
		t.Fatal()
	}
	// No solution for currentTerm >= 2
	if _findNewerCommitIndex(2, 0) != 0 {
		t.Fatal()
	}
	if _findNewerCommitIndex(3, 0) != 0 {
		t.Fatal()
	}
}

func TestFindNewerCommitIndex_Figure8_CaseCAndE(t *testing.T) {
	ci, err := config.NewClusterInfo([]ServerId{"s1", "s2", "s3", "s4", "s5"}, "s1")
	if err != nil {
		t.Fatal(err)
	}

	// Figure 8, case (c)
	terms := []TermNo{1, 2, 4} // leader line for the case
	imle := log.TestUtil_NewInMemoryLog_WithTerms(terms)
	lvs, err := consensus_state.NewLeaderVolatileState(ci, LogIndex(len(terms)))
	if err != nil {
		t.Fatal(err)
	}

	_findNewerCommitIndex := func(currentTerm TermNo, commitIndex LogIndex) LogIndex {
		nci, err := consensus_state.FindNewerCommitIndex(ci, lvs, imle, currentTerm, commitIndex)
		if err != nil {
			t.Fatal(err)
		}
		return nci
	}

	// With matchIndex stuck at 0, there is no solution for any currentTerm
	if _findNewerCommitIndex(1, 0) != 0 {
		t.Fatal()
	}
	if _findNewerCommitIndex(2, 0) != 0 {
		t.Fatal()
	}
	if _findNewerCommitIndex(3, 0) != 0 {
		t.Fatal()
	}

	// match peers for Figure 8, case (c)
	err = lvs.SetMatchIndexAndNextIndex("s2", 2)
	if err != nil {
		t.Fatal(err)
	}
	err = lvs.SetMatchIndexAndNextIndex("s3", 2)
	if err != nil {
		t.Fatal(err)
	}
	err = lvs.SetMatchIndexAndNextIndex("s4", 1)
	if err != nil {
		t.Fatal(err)
	}
	err = lvs.SetMatchIndexAndNextIndex("s5", 1)
	if err != nil {
		t.Fatal(err)
	}

	// While we cannot be at currentTerm=1, it has solutions
	if _findNewerCommitIndex(1, 0) != 1 {
		t.Fatal()
	}
	if _findNewerCommitIndex(1, 1) != 0 {
		t.Fatal()
	}
	// While we cannot be at currentTerm=2, it has solutions
	if _findNewerCommitIndex(2, 0) != 2 {
		t.Fatal()
	}
	if _findNewerCommitIndex(2, 1) != 2 {
		t.Fatal()
	}
	// While we cannot be at currentTerm=3, it has no solution
	if _findNewerCommitIndex(3, 0) != 0 {
		t.Fatal()
	}
	if _findNewerCommitIndex(3, 1) != 0 {
		t.Fatal()
	}
	if _findNewerCommitIndex(3, 2) != 0 {
		t.Fatal()
	}
	// No solution for currentTerm >= 4
	if _findNewerCommitIndex(4, 0) != 0 {
		t.Fatal()
	}
	if _findNewerCommitIndex(4, 1) != 0 {
		t.Fatal()
	}
	if _findNewerCommitIndex(4, 2) != 0 {
		t.Fatal()
	}

	// match peers for Figure 8, case (e)
	err = lvs.SetMatchIndexAndNextIndex("s2", 3)
	if err != nil {
		t.Fatal(err)
	}
	err = lvs.SetMatchIndexAndNextIndex("s3", 3)
	if err != nil {
		t.Fatal(err)
	}
	err = lvs.SetMatchIndexAndNextIndex("s4", 1)
	if err != nil {
		t.Fatal(err)
	}
	err = lvs.SetMatchIndexAndNextIndex("s5", 1)
	if err != nil {
		t.Fatal(err)
	}

	// Now currentTerm = 4 has a solution
	if _findNewerCommitIndex(4, 0) != 3 {
		t.Fatal()
	}
	if _findNewerCommitIndex(4, 1) != 3 {
		t.Fatal()
	}
	if _findNewerCommitIndex(4, 2) != 3 {
		t.Fatal()
	}
	if _findNewerCommitIndex(4, 3) != 0 {
		t.Fatal()
	}
}

func TestFindNewerCommitIndex_Figure8_CaseEextended(t *testing.T) {
	ci, err := config.NewClusterInfo([]ServerId{"s1", "s2", "s3", "s4", "s5"}, "s1")
	if err != nil {
		t.Fatal(err)
	}

	// Figure 8, case (e) extended with extra term 4 entry at index 4
	terms := []TermNo{1, 2, 4, 4} // leader line for the case
	imle := log.TestUtil_NewInMemoryLog_WithTerms(terms)
	lvs, err := consensus_state.NewLeaderVolatileState(ci, LogIndex(len(terms)))
	if err != nil {
		t.Fatal(err)
	}

	_findNewerCommitIndex := func(currentTerm TermNo, commitIndex LogIndex) LogIndex {
		nci, err := consensus_state.FindNewerCommitIndex(ci, lvs, imle, currentTerm, commitIndex)
		if err != nil {
			t.Fatal(err)
		}
		return nci
	}

	// match peers
	err = lvs.SetMatchIndexAndNextIndex("s2", 4)
	if err != nil {
		t.Fatal(err)
	}
	err = lvs.SetMatchIndexAndNextIndex("s3", 4)
	if err != nil {
		t.Fatal(err)
	}
	err = lvs.SetMatchIndexAndNextIndex("s4", 1)
	if err != nil {
		t.Fatal(err)
	}
	err = lvs.SetMatchIndexAndNextIndex("s5", 1)
	if err != nil {
		t.Fatal(err)
	}

	// currentTerm = 4 has a solution
	// the highest match is returned
	if _findNewerCommitIndex(4, 0) != 4 {
		t.Fatal()
	}
	if _findNewerCommitIndex(4, 1) != 4 {
		t.Fatal()
	}
	if _findNewerCommitIndex(4, 2) != 4 {
		t.Fatal()
	}
	if _findNewerCommitIndex(4, 3) != 4 {
		t.Fatal()
	}

	// returns 0 if current commitIndex is the highest match
	if _findNewerCommitIndex(4, 4) != 0 {
		t.Fatal()
	}
}

func TestFindNewerCommitIndex_SOLO(t *testing.T) {
	ci, err := config.NewClusterInfo([]ServerId{"s1"}, "s1")
	if err != nil {
		t.Fatal(err)
	}

	terms := []TermNo{1, 2, 2, 2, 3, 3}
	imle := log.TestUtil_NewInMemoryLog_WithTerms(terms)
	lvs, err := consensus_state.NewLeaderVolatileState(ci, LogIndex(len(terms)))
	if err != nil {
		t.Fatal(err)
	}

	_findNewerCommitIndex := func(currentTerm TermNo, commitIndex LogIndex) LogIndex {
		nci, err := consensus_state.FindNewerCommitIndex(ci, lvs, imle, currentTerm, commitIndex)
		if err != nil {
			t.Fatal(err)
		}
		return nci
	}

	if nci := _findNewerCommitIndex(1, 0); nci != 1 {
		t.Fatal(nci)
	}
	if nci := _findNewerCommitIndex(1, 1); nci != 0 {
		t.Fatal(nci)
	}

	// the highest match is returned
	if nci := _findNewerCommitIndex(2, 0); nci != 4 {
		t.Fatal(nci)
	}
	if nci := _findNewerCommitIndex(2, 3); nci != 4 {
		t.Fatal(nci)
	}
	if nci := _findNewerCommitIndex(3, 1); nci != 6 {
		t.Fatal(nci)
	}

	// returns 0 if current commitIndex is the highest match
	if nci := _findNewerCommitIndex(2, 4); nci != 0 {
		t.Fatal(nci)
	}

}
