package leader_test

import (
	"reflect"
	"testing"

	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/config"
	"github.com/divtxt/raft/consensus/leader"
	"github.com/divtxt/raft/internal"
	"github.com/divtxt/raft/log"
)

func TestLeaderVolatileState(t *testing.T) {
	ci, err := config.NewClusterInfo([]ServerId{101, 102, 103}, 103)
	if err != nil {
		t.Fatal(err)
	}

	maes := &mockAESender{}

	lvs, err := leader.NewLeaderVolatileState(ci, 42, maes)
	if err != nil {
		t.Fatal(err)
	}

	// Initial state
	// #5.3-p8s4: When a leader first comes to power, it initializes
	// all nextIndex values to the index just after the last one in
	// its log (11 in Figure 7).
	expectedNextIndex := map[ServerId]LogIndex{101: 43, 102: 43}
	if !reflect.DeepEqual(lvs.NextIndex, expectedNextIndex) {
		t.Fatal(lvs.NextIndex)
	}
	expectedMatchIndex := map[ServerId]LogIndex{101: 0, 102: 0}
	if !reflect.DeepEqual(lvs.MatchIndex, expectedMatchIndex) {
		t.Fatal(lvs.MatchIndex)
	}

	// getNextIndex
	idx, err := lvs.GetNextIndex(102)
	if err != nil {
		t.Fatal(err)
	}
	if idx != 43 {
		t.Fatal()
	}

	idx, err = lvs.GetNextIndex(105)
	if err.Error() != "LeaderVolatileState.GetNextIndex(): unknown peer: 105" {
		t.Fatal(err)
	}

	// DecrementNextIndex
	err = lvs.DecrementNextIndex(102)
	if err != nil {
		t.Fatal(err)
	}
	err = lvs.DecrementNextIndex(103)
	if err.Error() != "LeaderVolatileState.DecrementNextIndex(): unknown peer: 103" {
		t.Fatal(err)
	}
	expectedNextIndex = map[ServerId]LogIndex{101: 43, 102: 42}
	if !reflect.DeepEqual(lvs.NextIndex, expectedNextIndex) {
		t.Fatal(lvs.NextIndex)
	}
	idx, err = lvs.GetNextIndex(102)
	if err != nil {
		t.Fatal(err)
	}
	if idx != 42 {
		t.Fatal()
	}

	lvs.NextIndex[101] = 1
	err = lvs.DecrementNextIndex(101)
	if err.Error() != "LeaderVolatileState.DecrementNextIndex(): nextIndex <=1 for peer: 101" {
		t.Fatal(err)
	}

	// SetMatchIndexAndNextIndex
	err = lvs.SetMatchIndexAndNextIndex(102, 24)
	if err != nil {
		t.Fatal(err)
	}
	expectedNextIndex = map[ServerId]LogIndex{101: 1, 102: 25}
	if !reflect.DeepEqual(lvs.NextIndex, expectedNextIndex) {
		t.Fatal(lvs.NextIndex)
	}
	expectedMatchIndex = map[ServerId]LogIndex{101: 0, 102: 24}
	if !reflect.DeepEqual(lvs.MatchIndex, expectedMatchIndex) {
		t.Fatal(lvs.MatchIndex)
	}
	err = lvs.SetMatchIndexAndNextIndex(102, 0)
	if err != nil {
		t.Fatal(err)
	}
	expectedNextIndex = map[ServerId]LogIndex{101: 1, 102: 1}
	if !reflect.DeepEqual(lvs.NextIndex, expectedNextIndex) {
		t.Fatal(lvs.NextIndex)
	}
	expectedMatchIndex = map[ServerId]LogIndex{101: 0, 102: 0}
	if !reflect.DeepEqual(lvs.MatchIndex, expectedMatchIndex) {
		t.Fatal(lvs.MatchIndex)
	}

	// SendAppendEntriesToPeerAsync
	err = lvs.SendAppendEntriesToPeerAsync(102, false, 13, 1)
	if err != nil {
		t.Fatal(err)
	}
	expectedParams := internal.SendAppendEntriesParams{
		102,
		1,
		false,
		13,
		1,
	}
	if *maes.params != expectedParams {
		t.Fatal(maes.params)
	}
	maes.params = nil
	err = lvs.SendAppendEntriesToPeerAsync(105, false, 13, 1)
	if err.Error() != "LeaderVolatileState.GetNextIndex(): unknown peer: 105" {
		t.Fatal(err)
	}
	if maes.params != nil {
		t.Fatal(maes.params)
	}
}

// #RFS-L4: If there exists an N such that N > commitIndex, a majority
// of matchIndex[i] >= N, and log[N].term == currentTerm:
// set commitIndex = N (#5.3, #5.4)
func TestFindNewerCommitIndex_Figure8_CaseA(t *testing.T) {
	ci, err := config.NewClusterInfo([]ServerId{101, 102, 103, 104, 105}, 101)
	if err != nil {
		t.Fatal(err)
	}

	// Figure 8, case (a)
	terms := []TermNo{1, 2} // leader line for the case
	imle := log.TestUtil_NewInMemoryLog_WithTerms(terms, 3)
	lvs, err := leader.NewLeaderVolatileState(ci, LogIndex(len(terms)), nil)
	if err != nil {
		t.Fatal(err)
	}

	_findNewerCommitIndex := func(currentTerm TermNo, commitIndex LogIndex) LogIndex {
		nci, err := leader.FindNewerCommitIndex(ci, lvs, imle, currentTerm, commitIndex)
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
	err = lvs.SetMatchIndexAndNextIndex(102, 2)
	if err != nil {
		t.Fatal(err)
	}
	err = lvs.SetMatchIndexAndNextIndex(103, 1)
	if err != nil {
		t.Fatal(err)
	}
	err = lvs.SetMatchIndexAndNextIndex(104, 1)
	if err != nil {
		t.Fatal(err)
	}
	err = lvs.SetMatchIndexAndNextIndex(105, 1)
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
	ci, err := config.NewClusterInfo([]ServerId{101, 102, 103, 104, 105}, 101)
	if err != nil {
		t.Fatal(err)
	}

	// Figure 8, case (c)
	terms := []TermNo{1, 2, 4} // leader line for the case
	imle := log.TestUtil_NewInMemoryLog_WithTerms(terms, 3)
	lvs, err := leader.NewLeaderVolatileState(ci, LogIndex(len(terms)), nil)
	if err != nil {
		t.Fatal(err)
	}

	_findNewerCommitIndex := func(currentTerm TermNo, commitIndex LogIndex) LogIndex {
		nci, err := leader.FindNewerCommitIndex(ci, lvs, imle, currentTerm, commitIndex)
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
	err = lvs.SetMatchIndexAndNextIndex(102, 2)
	if err != nil {
		t.Fatal(err)
	}
	err = lvs.SetMatchIndexAndNextIndex(103, 2)
	if err != nil {
		t.Fatal(err)
	}
	err = lvs.SetMatchIndexAndNextIndex(104, 1)
	if err != nil {
		t.Fatal(err)
	}
	err = lvs.SetMatchIndexAndNextIndex(105, 1)
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
	err = lvs.SetMatchIndexAndNextIndex(102, 3)
	if err != nil {
		t.Fatal(err)
	}
	err = lvs.SetMatchIndexAndNextIndex(103, 3)
	if err != nil {
		t.Fatal(err)
	}
	err = lvs.SetMatchIndexAndNextIndex(104, 1)
	if err != nil {
		t.Fatal(err)
	}
	err = lvs.SetMatchIndexAndNextIndex(105, 1)
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
	ci, err := config.NewClusterInfo([]ServerId{101, 102, 103, 104, 105}, 101)
	if err != nil {
		t.Fatal(err)
	}

	// Figure 8, case (e) extended with extra term 4 entry at index 4
	terms := []TermNo{1, 2, 4, 4} // leader line for the case
	imle := log.TestUtil_NewInMemoryLog_WithTerms(terms, 3)
	lvs, err := leader.NewLeaderVolatileState(ci, LogIndex(len(terms)), nil)
	if err != nil {
		t.Fatal(err)
	}

	_findNewerCommitIndex := func(currentTerm TermNo, commitIndex LogIndex) LogIndex {
		nci, err := leader.FindNewerCommitIndex(ci, lvs, imle, currentTerm, commitIndex)
		if err != nil {
			t.Fatal(err)
		}
		return nci
	}

	// match peers
	err = lvs.SetMatchIndexAndNextIndex(102, 4)
	if err != nil {
		t.Fatal(err)
	}
	err = lvs.SetMatchIndexAndNextIndex(103, 4)
	if err != nil {
		t.Fatal(err)
	}
	err = lvs.SetMatchIndexAndNextIndex(104, 1)
	if err != nil {
		t.Fatal(err)
	}
	err = lvs.SetMatchIndexAndNextIndex(105, 1)
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
	ci, err := config.NewClusterInfo([]ServerId{101}, 101)
	if err != nil {
		t.Fatal(err)
	}

	terms := []TermNo{1, 2, 2, 2, 3, 3}
	imle := log.TestUtil_NewInMemoryLog_WithTerms(terms, 3)
	lvs, err := leader.NewLeaderVolatileState(ci, LogIndex(len(terms)), nil)
	if err != nil {
		t.Fatal(err)
	}

	_findNewerCommitIndex := func(currentTerm TermNo, commitIndex LogIndex) LogIndex {
		nci, err := leader.FindNewerCommitIndex(ci, lvs, imle, currentTerm, commitIndex)
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

type mockAESender struct {
	params *internal.SendAppendEntriesParams
}

func (maes *mockAESender) SendAppendEntriesToPeerAsync(
	params internal.SendAppendEntriesParams,
) {
	if maes.params != nil {
		panic("more than one call!")
	}
	maes.params = &params
}
