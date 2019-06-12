package committer

import (
	"sync"
	"testing"

	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/log"
	"github.com/divtxt/raft/logindex"
	"github.com/divtxt/raft/testhelpers"
)

// #RFS-A1: If commitIndex > lastApplied: increment lastApplied, apply
// log[lastApplied] to state machine (#5.3)
func TestCommitter(t *testing.T) {
	iml, err := log.TestUtil_NewInMemoryLog_WithFigure7LeaderLine(3)
	if err != nil {
		t.Fatal(err)
	}
	err = iml.DiscardEntriesBeforeIndex(2)
	if err != nil {
		t.Fatal(err)
	}

	dsm := testhelpers.NewDummyStateMachine(3)
	commitIndex := logindex.NewWatchedIndex(&sync.Mutex{})

	committer := NewCommitter(iml, commitIndex, dsm, nil)

	// Increasing commitIndex should trigger run that drives commits
	err = commitIndex.UnsafeSet(4)
	if err != nil {
		t.Fatal(err)
	}

	// Test StopSync() - this also waits for the first run to complete.
	committer.StopSync()
	if dsm.GetLastApplied() != 4 {
		t.Fatal()
	}
	// Only entries after lastApplied should be applied.
	if !dsm.AppliedCommandsEqual(4) {
		t.Fatal()
	}

	// Enable TriggeredRunner test mode for the rest of the test.
	committer.commitApplier.TestHelperFakeRestart()

	// GetResultAsync for committed index should be an error
	_, err = committer.GetResultAsync(4)
	if err.Error() != "FATAL: logIndex=4 is <= commitIndex=4" {
		t.Fatal(err)
	}

	// GetResultAsync for new notifications.
	// Intentionally not registering for some indexes to test that gaps are allowed.
	// We're cheating a bit here in this test since these entries are already in the log.
	crc6, err := committer.GetResultAsync(6)
	if err != nil {
		t.Fatal(err)
	}
	if crc6 == nil {
		t.Fatal()
	}
	crc8, err := committer.GetResultAsync(8)
	if err != nil {
		t.Fatal(err)
	}
	if crc8 == nil {
		t.Fatal()
	}
	crc9, err := committer.GetResultAsync(9)
	if err != nil {
		t.Fatal(err)
	}
	if crc9 == nil {
		t.Fatal()
	}
	crc10, err := committer.GetResultAsync(10)
	if err != nil {
		t.Fatal(err)
	}
	if crc10 == nil {
		t.Fatal()
	}
	testhelpers.AssertWillBlock(crc6)
	testhelpers.AssertWillBlock(crc8)
	testhelpers.AssertWillBlock(crc9)
	testhelpers.AssertWillBlock(crc10)

	// GetResultAsync for an older index should be an error
	_, err = committer.GetResultAsync(7)
	if err.Error() != "FATAL: logIndex=7 is <= highestRegisteredIndex=10" {
		t.Fatal(err)
	}

	// Advancing commitIndex by multiple values should drive as many commits
	// and notify relevant listeners with the results.
	err = commitIndex.UnsafeSet(8)
	if err != nil {
		t.Fatal(err)
	}
	if !committer.commitApplier.TestHelperRunOnceIfTriggerPending() {
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

	// Regressing commitIndex should be an error
	err = commitIndex.UnsafeSet(7)
	if err.Error() != "FATAL: newCi=7 is < oldCi=8" {
		t.Fatal(err)
	}
	// reset for rest of tests :P
	committer.cachedCommitIndex = 7
	err = commitIndex.UnsafeSet(8)
	if err != nil {
		t.Fatal(err)
	}

	// Add some more log entries...
	_, err = iml.AppendEntry(LogEntry{8, Command("c11")})
	if err != nil {
		t.Fatal(err)
	}
	_, err = iml.AppendEntry(LogEntry{8, Command("c12")})
	if err != nil {
		t.Fatal(err)
	}
	ioleC13, err := iml.AppendEntry(LogEntry{8, Command("c13")})
	if err != nil {
		t.Fatal(err)
	}
	if ioleC13 != 13 {
		t.Fatal(ioleC13)
	}

	// GetResultAsync a few more entries.
	crc12, err := committer.GetResultAsync(12)
	if err != nil {
		t.Fatal(err)
	}
	if crc12 == nil {
		t.Fatal()
	}
	testhelpers.AssertWillBlock(crc12)

	// Discard with later index should not affect highestRegisteredIndex.
	crc13, err := committer.GetResultAsync(13)
	if err != nil {
		t.Fatal(err)
	}
	if crc13 == nil {
		t.Fatal()
	}
	testhelpers.AssertWillBlock(crc13)

	// Allowed index for register should have moved up
	_, err = committer.GetResultAsync(12)
	if err.Error() != "FATAL: logIndex=12 is <= highestRegisteredIndex=13" {
		t.Fatal(err)
	}

	// Discard should close only relevant channels
	err = iml.SetEntriesAfterIndex(9, nil)
	if err != nil {
		t.Fatal(err)
	}
	testhelpers.AssertWillBlock(crc9)
	testhelpers.AssertIsClosed(crc10)
	testhelpers.AssertIsClosed(crc12)
	testhelpers.AssertIsClosed(crc13)

	// GetResultAsync of indexOfLastEntry after discard is not allowed
	_, err = committer.GetResultAsync(9)
	if err.Error() != "FATAL: logIndex=9 is <= highestRegisteredIndex=9" {
		t.Fatal(err)
	}
	// GetResultAsync beyond indexOfLastEntry after discard is not allowed
	_, err = committer.GetResultAsync(10)
	if err.Error() != "FATAL: logIndex=10 is > indexOfLastEntry=9" {
		t.Fatal(err)
	}
	testhelpers.AssertWillBlock(crc9)

	// Adding entries and advancing commitIndex should drive new commits.
	ioleC10b, err := iml.AppendEntry(LogEntry{9, Command("c10")})
	if ioleC10b != 10 || err != nil {
		t.Fatal(10, err)
	}
	crc10b, err := committer.GetResultAsync(10)
	if err != nil {
		t.Fatal(err)
	}
	err = commitIndex.UnsafeSet(9)
	if err != nil {
		t.Fatal(err)
	}
	err = commitIndex.UnsafeSet(10)
	if err != nil {
		t.Fatal(err)
	}
	if !committer.commitApplier.TestHelperRunOnceIfTriggerPending() {
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

	// Committing past the end of the log should be an error
	err = commitIndex.UnsafeSet(14)
	if err.Error() != "FATAL: commitIndex=14 is > indexOfLastEntry=10" {
		t.Fatal(err)
	}
}

// TODO: tests for fceListener
