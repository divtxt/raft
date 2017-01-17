package committer

import (
	"fmt"
	"sync/atomic"

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
	_commitIndex       LogIndex
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
	// Concurrency analysis:
	// - PassiveConsensusModule will only make one call at a time to this method
	// - this is the only method that modifies commitIndex
	// Because of the above, simple atomic get and set actions are sufficient, and no
	// extended locking is needed.

	currentCI := c.atomicGetCommitIndex()

	// Check commitIndex is not going backward
	if currentCI > commitIndex {
		panic(fmt.Sprintf("FATAL: decreasing commitIndex: %v > %v", currentCI, commitIndex))
	}

	// Update commitIndex and then trigger a run of the applier goroutine
	c.atomicSetCommitIndex(commitIndex)
	c.commitApplier.TriggerRun()
}

func (c *Committer) atomicGetCommitIndex() LogIndex {
	return LogIndex(atomic.LoadUint64((*uint64)(&c._commitIndex)))
}

func (c *Committer) atomicSetCommitIndex(commitIndex LogIndex) {
	atomic.StoreUint64((*uint64)(&c._commitIndex), uint64(commitIndex))
}

// Apply pending committed entries.
func (c *Committer) applyPendingCommits() {
	currentCI := c.atomicGetCommitIndex()

	for c.appliedCommitIndex < currentCI {
		// #RFS-A1: If commitIndex > lastApplied: increment lastApplied, apply
		// log[lastApplied] to state machine (#5.3)
		// TODO: get and apply multiple entries at a time
		c.applyOnePendingCommit()
	}
}

// Apply one pending commit.
func (c *Committer) applyOnePendingCommit() {
	currentCI := c.atomicGetCommitIndex()

	if c.appliedCommitIndex >= currentCI {
		return
	}

	// Get one command from the raft log
	indexToApply := c.appliedCommitIndex + 1
	entries, err := c.log.GetEntriesAfterIndex(indexToApply-1, 1)
	if err != nil {
		panic(err)
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
