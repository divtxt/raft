package raft

import (
	"reflect"
	"testing"
)

func TestLeaderVolatileState(t *testing.T) {
	ci := NewClusterInfo([]ServerId{"s1", "s2", "s3"}, "s3")

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
}
