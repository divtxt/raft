package committer

import (
	"testing"

	"github.com/divtxt/raft/log"
	"github.com/divtxt/raft/testhelpers"
)

// #RFS-A1: If commitIndex > lastApplied: increment lastApplied, apply
// log[lastApplied] to state machine (#5.3)
func TestCommitter(t *testing.T) {
	iml := log.TestUtil_NewInMemoryLog_WithFigure7LeaderLine()
	dsm := testhelpers.NewDummyStateMachine(3)

	committer := NewCommitter(iml, dsm)

	// CommitIndexChanged should trigger run that drives commits
	committer.CommitIndexChanged(4)

	// Test StopSync() - this also waits for the first run to complete.
	committer.StopSync()
	if dsm.GetLastApplied() != 4 {
		t.Fatal()
	}
	if !dsm.AppliedCommandsEqual(4) {
		t.Fatal()
	}

	// Enable TriggeredRunner test mode
	committer.commitApplier.TestHelperFakeRestart()

	// Advancing commitIndex by multiple values should drive as many commits
	committer.CommitIndexChanged(8)
	if !committer.commitApplier.TestHelperRunOnceIfTriggerPending() {
		t.Fatal()
	}
	if dsm.GetLastApplied() != 8 {
		t.Fatal()
	}
	if !dsm.AppliedCommandsEqual(4, 5, 6, 7, 8) {
		t.Fatal()
	}

	// Regressing commitIndex should panic
	testhelpers.TestHelper_ExpectPanic(
		t,
		func() { committer.CommitIndexChanged(7) },
		"FATAL: decreasing commitIndex: 8 > 7",
	)
}
