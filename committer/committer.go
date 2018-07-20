package committer

import (
	"fmt"
	"sync"

	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/internal"
	"github.com/divtxt/raft/util"
)

type FatalErrorHandler func(err error)

// Committer is an implementation of the ICommitter internal interface.
//
// It is a goroutine that applies committed log entries to the state machine
// and notifying clients that are waiting for those entries to be committed.
//
type Committer struct {
	mutex sync.Mutex

	stopRequest bool

	// -- Commit state
	// commitIndex is the index of highest log entry known to be committed
	// (initialized to 0, increases monotonically)
	commitIndex LogIndex

	// -- External components
	log          internal.LogTailOnlyRO
	stateMachine StateMachine
	feHandler    FatalErrorHandler

	// -- Internal components
	commitApplier *util.TriggeredRunner

	// -- Commit listeners
	listeners              map[LogIndex]chan CommandResult
	highestRegisteredIndex LogIndex
}

// NewCommitter creates a new Committer with the given parameters.
//
// A goroutine is started that applies committed log entries to the state machine and notifies
// clients that are waiting for those entries to be committed.
//
// If the goroutine sees an error this is fatal for it, and it will call fceListener instead of
// calling panic(). fceListener is expected to call the committer's StopSync() method before it
// returns!
//
func NewCommitter(
	log internal.LogTailOnlyRO,
	stateMachine StateMachine,
	feHandler FatalErrorHandler,
) *Committer {
	// TODO: check that parameters are not nil?!

	c := &Committer{
		mutex:                  sync.Mutex{},
		stopRequest:            false,
		commitIndex:            0,
		log:                    log,
		stateMachine:           stateMachine,
		feHandler:              feHandler,
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
	c.mutex.Lock()
	c.stopRequest = true
	c.mutex.Unlock()

	c.commitApplier.StopSync()
}

// ---- Implement ICommitter

func (c *Committer) RegisterListener(logIndex LogIndex) (<-chan CommandResult, error) {
	iole, err := c.log.GetIndexOfLastEntry()
	if err != nil {
		return nil, err
	}
	if logIndex > iole {
		return nil, fmt.Errorf(
			"FATAL: logIndex=%v is > current iole=%v", logIndex, iole,
		)
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	if logIndex <= c.commitIndex {
		return nil, fmt.Errorf(
			"FATAL: logIndex=%v is <= commitIndex=%v", logIndex, c.commitIndex,
		)
	}
	if logIndex <= c.highestRegisteredIndex {
		return nil, fmt.Errorf(
			"FATAL: logIndex=%v is <= highestRegisteredIndex=%v",
			logIndex,
			c.highestRegisteredIndex,
		)
	}

	crc := make(chan CommandResult, 1)

	c.listeners[logIndex] = crc
	c.highestRegisteredIndex = logIndex

	return crc, nil
}

func (c *Committer) RemoveListenersAfterIndex(afterIndex LogIndex) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for i := afterIndex + 1; i <= c.highestRegisteredIndex; i++ {
		crc, ok := c.listeners[i]
		if ok {
			delete(c.listeners, i)
			close(crc)
		}
	}

	if afterIndex < c.highestRegisteredIndex {
		c.highestRegisteredIndex = afterIndex
	}

	return nil
}

// Commit log entries to the state machine asynchronously up to the given index.
//
// Commits are applied asynchronously by the Committer's goroutine.
//
// Will return an error if commitIndex decreases or if StopSync has been called.
func (c *Committer) CommitAsync(commitIndex LogIndex) error {
	iole, err := c.log.GetIndexOfLastEntry()
	if err != nil {
		return err
	}
	if commitIndex > iole {
		return fmt.Errorf(
			"FATAL: commitIndex=%v is > current iole=%v", commitIndex, iole,
		)
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Check commitIndex is not going backward
	if commitIndex < c.commitIndex {
		return fmt.Errorf(
			"FATAL: commitIndex=%v is < current commitIndex=%v", commitIndex, c.commitIndex,
		)
	}

	// Update commitIndex and then trigger a run of the applier goroutine
	c.commitIndex = commitIndex
	c.commitApplier.TriggerRun()

	return nil
}

// ----

// Apply pending committed entries.
func (c *Committer) applyPendingCommits() {
	// Concurrency:
	// - there is only one method to this call at a time
	// - commitIndex can only increase, so we can snapshot it as a low value

	var stopRequestSnapshot bool
	var commitIndexSnapshot LogIndex

	for {
		// safely get commitIndex
		c.mutex.Lock()
		stopRequestSnapshot = c.stopRequest
		commitIndexSnapshot = c.commitIndex
		c.mutex.Unlock()

		// return early if we're trying to stop
		if stopRequestSnapshot {
			return
		}

		lastApplied := c.stateMachine.GetLastApplied()

		// Return if no more entries to apply at this time.
		// (TriggeredRunner should call again if CommitAsync advanced commitIndex)
		if lastApplied >= commitIndexSnapshot {
			return
		}

		// Get a batch of entries from the raft log.
		entries, err := c.log.GetEntriesAfterIndex(lastApplied)
		if err != nil {
			c.feHandler(err)
			return
		}

		// Apply the entries to the state machine.
		for _, entry := range entries {
			// Calculate the index that of the entry we are going to apply
			indexToApply := lastApplied + 1

			// Return if next entry is past the commitIndex
			// (TriggeredRunner should call again if CommitAsync advanced commitIndex)
			if indexToApply > commitIndexSnapshot {
				return
			}

			// Get the commit listener for this index
			c.mutex.Lock()
			stopRequestSnapshot = c.stopRequest
			// since we have the mutex, update our copy of commitIndex
			commitIndexSnapshot = c.commitIndex
			crc, haveCrc := c.listeners[indexToApply]
			if haveCrc {
				delete(c.listeners, indexToApply)
			}
			c.mutex.Unlock()

			// return early if we're trying to stop
			if stopRequestSnapshot {
				return
			}

			// Apply the command to the state machine.
			commandResult := c.stateMachine.ApplyCommand(indexToApply, entry.Command)

			// Send the result to the commit listener.
			if haveCrc {
				crc <- commandResult
			}

			// The index of the entry we have just applied MUST be the new value of lastApplied.
			lastApplied = indexToApply
		}
	}
}
