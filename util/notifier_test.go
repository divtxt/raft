package util_test

import (
	"testing"

	"github.com/divtxt/raft/testhelpers"
	. "github.com/divtxt/raft/util"
)

func TestCommitNotifier(t *testing.T) {
	n := NewCommitNotifier(5)
	if n.GetCommitIndex() != 5 {
		t.Fatal(n)
	}

	// Registering for committed index should panic
	testhelpers.TestHelper_ExpectPanicMessage(
		t,
		func() {
			n.RegisterListener(5)
		},
		"FATAL: logIndex=5 is <= commitIndex=5",
	)

	// Register for new notifications
	l6 := n.RegisterListener(6)
	if l6 == nil {
		t.Fatal()
	}
	l7 := n.RegisterListener(7)
	if l7 == nil {
		t.Fatal()
	}
	l8 := n.RegisterListener(8)
	if l8 == nil {
		t.Fatal()
	}
	l9 := n.RegisterListener(9)
	if l8 == nil {
		t.Fatal()
	}
	testhelpers.AssertWillBlock(l6)
	testhelpers.AssertWillBlock(l7)
	testhelpers.AssertWillBlock(l8)
	testhelpers.AssertWillBlock(l9)

	// Registering for same index should panic
	testhelpers.TestHelper_ExpectPanicMessage(
		t,
		func() {
			n.RegisterListener(7)
		},
		"FATAL: replyChan already exists for logIndex=7",
	)

	// Advancing commitIndex should notify relevant listeners
	if n.GetCommitIndex() != 5 {
		t.Fatal(n)
	}
	n.CommitIndexChanged(6)
	if n.GetCommitIndex() != 6 {
		t.Fatal(n)
	}
	testhelpers.AssertHasValue(l6)
	testhelpers.AssertWillBlock(l7)
	testhelpers.AssertWillBlock(l8)
	testhelpers.AssertWillBlock(l9)

	// Clear should close newer listeners
	n.ClearListenersAfterIndex(7)
	testhelpers.AssertWillBlock(l7)
	testhelpers.AssertClosed(l8)
	testhelpers.AssertClosed(l9)

	// Can have new listeners with a gap
	l12 := n.RegisterListener(12)
	if l12 == nil {
		t.Fatal()
	}
	testhelpers.AssertWillBlock(l12)

	// Advancing commitIndex should notify relevant listeners
	n.CommitIndexChanged(12)
	if n.GetCommitIndex() != 12 {
		t.Fatal(n)
	}
	testhelpers.AssertHasValue(l7)
	testhelpers.AssertHasValue(l12)
}
