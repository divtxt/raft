package committer

import (
	"fmt"

	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/util"
)

// Committer is a goroutine that applies committed log entries to the state machine.
//
// #RFS-A1: If commitIndex > lastApplied: increment lastApplied, apply
// log[lastApplied] to state machine (#5.3)
//
type Committer struct {
	// -- Commit state
	// commitIndex is the index of highest log entry known to be committed
	// (initialized to 0, increases monotonically)
	commitIndex        LogIndex
	appliedCommitIndex LogIndex

	// -- External components
	log           LogReadOnly
	stateMachine  StateMachine
	commitApplier *util.TriggeredRunner
}

// NewCommitter creates a new Committer with the given parameters.
func NewCommitter(log LogReadOnly, stateMachine StateMachine) *Committer {
	c := &Committer{
		0,
		stateMachine.GetLastApplied(),
		log,
		stateMachine,
		nil, // commitApplier
	}
	c.commitApplier = util.NewTriggeredRunner(c.applyPendingCommits)

	return c
}

// StopSync will stop the Committer's goroutine.
//
// Will panic if called more than once.
func (c *Committer) StopSync() {
	c.commitApplier.StopSync()
}

// Receive raft commit index changes.
//
// Commits are applied asynchronously by the Committer's goroutine.
//
// Will panic if commitIndex decreases or if StopSync has been called.
func (c *Committer) CommitIndexChanged(commitIndex LogIndex) {
	// FIXME: mutex

	// Check commitIndex is not going backward
	if c.commitIndex > commitIndex {
		panic(fmt.Sprintf("FATAL: decreasing commitIndex: %v > %v", c.commitIndex, commitIndex))
	}

	// Update commitIndex and then trigger a run of the applier goroutine
	c.commitIndex = commitIndex
	c.commitApplier.TriggerRun()
}

// Apply pending committed entries.
func (c *Committer) applyPendingCommits() {
	for c.appliedCommitIndex < c.commitIndex {
		// #RFS-A1: If commitIndex > lastApplied: increment lastApplied, apply
		// log[lastApplied] to state machine (#5.3)
		// TODO: get and apply multiple entries at a time
		c.applyOnePendingCommit()
	}
}

// Apply one pending commit.
func (c *Committer) applyOnePendingCommit() {
	if c.appliedCommitIndex >= c.commitIndex {
		return
	}

	// Get one command from the raft log
	indexToApply := c.appliedCommitIndex + 1
	entries, err := c.log.GetEntriesAfterIndex(indexToApply-1, 1)
	if err != nil {
		panic(err) // FIXME: non-panic error handling
	}
	commandToApply := entries[0].Command

	// Apply the command to the state machine.
	c.stateMachine.ApplyCommand(indexToApply, commandToApply)

	// Update internal state
	c.appliedCommitIndex = indexToApply
}

// For test use only!
func (c *Committer) TestHelperGetCommitApplier() *util.TriggeredRunner {
	return c.commitApplier
}
