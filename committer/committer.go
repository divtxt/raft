package committer

import (
	"fmt"
	"sync"
	"sync/atomic"

	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/util"
)

// Committer is an implementation of the ICommitter internal interface.
//
// It is a goroutine that applies committed log entries to the state machine
// and notifying clients that are waiting for those entries to be committed.
//
type Committer struct {
	mutex sync.Mutex

	// -- Commit state
	// commitIndex is the index of highest log entry known to be committed
	// (initialized to 0, increases monotonically)
	commitIndex  LogIndex
	_lastApplied LogIndex

	// -- External components
	log           LogReadOnly
	stateMachine  StateMachine
	commitApplier *util.TriggeredRunner

	// -- Commit listeners
	listeners              map[LogIndex]chan CommandResult
	highestRegisteredIndex LogIndex
}

// NewCommitter creates a new Committer with the given parameters.
func NewCommitter(log LogReadOnly, stateMachine StateMachine) *Committer {
	c := &Committer{
		mutex:                  sync.Mutex{},
		commitIndex:            0,
		_lastApplied:           stateMachine.GetLastApplied(),
		log:                    log,
		stateMachine:           stateMachine,
		commitApplier:          nil,
		listeners:              make(map[LogIndex]chan CommandResult),
		highestRegisteredIndex: 0,
	}
	c.commitApplier = util.NewTriggeredRunner(c.applyPendingCommits)

	return c
}

// StopSync will stop the Committer's goroutine.
//
// Will panic if called more than once.
func (c *Committer) StopSync() {
	// FIXME: should other methods be checking stopped state?
	c.commitApplier.StopSync()
}

// ---- Implement ICommitter

func (c *Committer) RegisterListener(logIndex LogIndex) <-chan CommandResult {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if logIndex <= c.commitIndex {
		panic(fmt.Sprintf("FATAL: logIndex=%v is <= commitIndex=%v", logIndex, c.commitIndex))
	}
	if logIndex <= c.highestRegisteredIndex {
		panic(fmt.Sprintf(
			"FATAL: logIndex=%v is <= highestRegisteredIndex=%v",
			logIndex,
			c.highestRegisteredIndex,
		))
	}

	crc := make(chan CommandResult, 1)

	c.listeners[logIndex] = crc
	c.highestRegisteredIndex = logIndex

	return crc
}

func (c *Committer) RemoveListenersAfterIndex(afterIndex LogIndex) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for i := afterIndex + 1; i <= c.highestRegisteredIndex; i++ {
		crc, ok := c.listeners[i]
		if ok {
			delete(c.listeners, i)
			close(crc)
		}
	}

	c.highestRegisteredIndex = afterIndex
}

// Commit log entries to the state machine asynchronously up to the given index.
//
// Commits are applied asynchronously by the Committer's goroutine.
//
// Will panic if commitIndex decreases or if StopSync has been called.
func (c *Committer) CommitAsync(commitIndex LogIndex) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Check commitIndex is not going backward
	if commitIndex < c.commitIndex {
		panic(fmt.Sprintf(
			"FATAL: commitIndex=%v is < current commitIndex=%v", commitIndex, c.commitIndex,
		))
	}

	// Update commitIndex and then trigger a run of the applier goroutine
	c.commitIndex = commitIndex
	c.commitApplier.TriggerRun()
}

// ----

func (c *Committer) _atomicGetCommitIndex() LogIndex {
	return LogIndex(atomic.LoadUint64((*uint64)(&c.commitIndex)))
}

func (c *Committer) _atomicSetCommitIndex(commitIndex LogIndex) {
	atomic.StoreUint64((*uint64)(&c.commitIndex), uint64(commitIndex))
}

// Apply pending committed entries.
func (c *Committer) applyPendingCommits() {
	// Concurrency:
	// - there is only one method to this call at a time
	// - only this method drives changes to _lastApplied, so it does not need locking
	// - commitIndex can only increase, so we can snapshot it as a low value

	for {
		// safely get commitIndex
		c.mutex.Lock()
		commitIndexSnapshot := c.commitIndex
		c.mutex.Unlock()

		// Return if no more entries to apply at this time.
		// (TriggeredRunner should call again if CommitAsync advanced commitIndex)
		if c._lastApplied >= commitIndexSnapshot {
			return
		}

		// Apply one entry.
		// TODO: get and apply multiple entries at a time
		indexToApply := c._lastApplied + 1
		c.applyOnePendingCommit(indexToApply)
		c._lastApplied = indexToApply
	}
}

// Apply one pending commit.
// Assumes lastApplied has not crossed commitIndex!
func (c *Committer) applyOnePendingCommit(indexToApply LogIndex) {
	// Concurrency: see notes above for applyPendingCommits

	// Get command from the raft log
	entries, err := c.log.GetEntriesAfterIndex(indexToApply-1, 1)
	if err != nil {
		panic(err)
	}
	commandToApply := entries[0].Command

	// Apply the command to the state machine.
	c.stateMachine.ApplyCommand(indexToApply, commandToApply)
	result := (1000 + indexToApply) // FIXME

	// Send the result to the commit listener.
	c.mutex.Lock()
	defer c.mutex.Unlock()
	crc, ok := c.listeners[indexToApply]
	if ok {
		delete(c.listeners, indexToApply)
		crc <- result
	}
}

// For test use only!
func (c *Committer) TestHelperGetCommitApplier() *util.TriggeredRunner {
	return c.commitApplier
}
