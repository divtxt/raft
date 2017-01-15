package committer

import (
	"bytes"
	"fmt"
	"strconv"
	"testing"

	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/log"
	"github.com/divtxt/raft/testhelpers"
)

// #RFS-A1: If commitIndex > lastApplied: increment lastApplied, apply
// log[lastApplied] to state machine (#5.3)
func TestCommitter(t *testing.T) {
	iml := log.TestUtil_NewInMemoryLog_WithFigure7LeaderLine()
	dsm := NewDummyStateMachine(3)

	committer := NewCommitter(iml, dsm)

	// CommitIndexChanged should trigger run that drives commits
	committer.CommitIndexChanged(4)

	// Test StopSync() - this also waits for the first run to complete.
	committer.StopSync()
	if dsm.GetLastApplied() != 4 {
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

	// Regressing commitIndex should panic
	testhelpers.TestHelper_ExpectPanic(
		t,
		func() { committer.CommitIndexChanged(7) },
		"FATAL: decreasing commitIndex: 8 > 7",
	)
}

// -- MOVE TO DSM

// Dummy state machine that implements StateMachine.
// Does not provide any useful state or commands. Meant only for tests.
type DummyStateMachine struct {
	lastApplied LogIndex
}

// Will serialize to Command("cN")
func DummyCommand(N int) Command {
	return Command("c" + strconv.Itoa(N))
}

func NewDummyStateMachine(lastApplied LogIndex) *DummyStateMachine {
	return &DummyStateMachine{lastApplied}
}

func (dsm *DummyStateMachine) GetLastApplied() LogIndex {
	return dsm.lastApplied
}

func (dsm *DummyStateMachine) ApplyCommand(logIndex LogIndex, command Command) {
	if logIndex < dsm.lastApplied {
		panic(fmt.Sprintf(
			"DummyStateMachine: logIndex=%d is < current lastApplied=%d",
			logIndex,
			dsm.lastApplied,
		))
	}
	if !DummyCommandEquals(command, int(logIndex)) {
		panic(fmt.Sprintf(
			"DummyStateMachine: command=%v is wrong for logIndex=%v",
			command,
			logIndex,
		))
	}
	dsm.lastApplied = logIndex
}

// Helper
func DummyCommandEquals(c Command, n int) bool {
	cn := Command("c" + strconv.Itoa(n))
	return bytes.Equal(c, cn)
}
