package committer

import (
	"testing"

	"github.com/divtxt/raft/consensus"
	"github.com/divtxt/raft/log"
	"github.com/divtxt/raft/testhelpers"
)

// #RFS-A1: If commitIndex > lastApplied: increment lastApplied, apply
// log[lastApplied] to state machine (#5.3)
func TestCommitter(t *testing.T) {
	iml := log.TestUtil_NewInMemoryLog_WithFigure7LeaderLine()
	dsm := testhelpers.NewDummyStateMachine(3)

	committerImpl := NewCommitter(iml, dsm)
	var committer consensus.ICommitter = committerImpl

	// CommitAsync should trigger run that drives commits
	committer.CommitAsync(4)

	// Test StopSync() - this also waits for the first run to complete.
	committerImpl.StopSync()
	if dsm.GetLastApplied() != 4 {
		t.Fatal()
	}
	// Only entries after lastApplied should be applied.
	if !dsm.AppliedCommandsEqual(4) {
		t.Fatal()
	}

	// Enable TriggeredRunner test mode for the rest of the test.
	committerImpl.commitApplier.TestHelperFakeRestart()

	// Registering for committed index should panic
	testhelpers.TestHelper_ExpectPanicMessage(
		t,
		func() {
			committer.RegisterListener(4)
		},
		"FATAL: logIndex=4 is <= commitIndex=4",
	)

	// Register for new notifications.
	// Intentionally not registering for some indexes to test that gaps are allowed.
	// We're cheating a bit here in this test since these entries are already in the log.
	crc6 := committer.RegisterListener(6)
	if crc6 == nil {
		t.Fatal()
	}
	crc8 := committer.RegisterListener(8)
	if crc8 == nil {
		t.Fatal()
	}
	crc9 := committer.RegisterListener(9)
	if crc9 == nil {
		t.Fatal()
	}
	crc10 := committer.RegisterListener(10)
	if crc10 == nil {
		t.Fatal()
	}
	testhelpers.AssertWillBlock(crc6)
	testhelpers.AssertWillBlock(crc8)
	testhelpers.AssertWillBlock(crc9)
	testhelpers.AssertWillBlock(crc10)

	// Trying to register for an older index should panic
	testhelpers.TestHelper_ExpectPanicMessage(
		t,
		func() {
			committer.RegisterListener(7)
		},
		"FATAL: logIndex=7 is <= highestRegisteredIndex=10",
	)

	// Advancing commitIndex by multiple values should drive as many commits
	// and notify relevant listeners with the results.
	committer.CommitAsync(8)
	if !committerImpl.commitApplier.TestHelperRunOnceIfTriggerPending() {
		t.Fatal()
	}
	if dsm.GetLastApplied() != 8 {
		t.Fatal()
	}
	if !dsm.AppliedCommandsEqual(4, 5, 6, 7, 8) {
		t.Fatal()
	}
	if v := testhelpers.GetCommandResult(crc6); v != "rc6" {
		t.Fatal(v)
	}
	if v := testhelpers.GetCommandResult(crc8); v != "rc8" {
		t.Fatal(v)
	}
	testhelpers.AssertWillBlock(crc9)
	testhelpers.AssertWillBlock(crc10)

	// Regressing commitIndex should panic
	testhelpers.TestHelper_ExpectPanic(
		t,
		func() { committer.CommitAsync(7) },
		"FATAL: commitIndex=7 is < current commitIndex=8",
	)

	// Register a few more listeners
	// We're cheating a bit here in this test since these entries are never put in the log.
	crc12 := committer.RegisterListener(12)
	if crc12 == nil {
		t.Fatal()
	}
	crc13 := committer.RegisterListener(13)
	if crc13 == nil {
		t.Fatal()
	}
	testhelpers.AssertWillBlock(crc12)
	testhelpers.AssertWillBlock(crc13)

	// Allowed index for register should have moved up
	testhelpers.TestHelper_ExpectPanicMessage(
		t,
		func() {
			committer.RegisterListener(12)
		},
		"FATAL: logIndex=12 is <= highestRegisteredIndex=13",
	)

	// Remove should close only relevant listeners
	committer.RemoveListenersAfterIndex(9)
	testhelpers.AssertWillBlock(crc9)
	testhelpers.AssertIsClosed(crc10)
	testhelpers.AssertIsClosed(crc12)
	testhelpers.AssertIsClosed(crc13)

	// Should now be allowed to register listeners after the remove index
	testhelpers.TestHelper_ExpectPanicMessage(
		t,
		func() {
			committer.RegisterListener(9)
		},
		"FATAL: logIndex=9 is <= highestRegisteredIndex=9",
	)
	crc10b := committer.RegisterListener(10)
	if crc10b == nil {
		t.Fatal()
	}
	testhelpers.AssertWillBlock(crc9)
	testhelpers.AssertWillBlock(crc10b)

	// Advancing commitIndex should drive new commits.
	// We're cheating a bit here in this test since we never removed and re-added entry at 10.
	committer.CommitAsync(9)
	committer.CommitAsync(10)
	if !committerImpl.commitApplier.TestHelperRunOnceIfTriggerPending() {
		t.Fatal()
	}
	if dsm.GetLastApplied() != 10 {
		t.Fatal()
	}
	if !dsm.AppliedCommandsEqual(4, 5, 6, 7, 8, 9, 10) {
		t.Fatal()
	}
	if v := testhelpers.GetCommandResult(crc9); v != "rc9" {
		t.Fatal(v)
	}
	if v := testhelpers.GetCommandResult(crc10b); v != "rc10" {
		t.Fatal(v)
	}
}
