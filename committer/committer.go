package committer

import (
	"fmt"
	"sync"

	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/internal"
	"github.com/divtxt/raft/util"
)

type FatalErrorHandler func(err error)

// A Committer is a goroutine that applies committed log entries to the state
// machine and notifies the commit listener for each entry.
//
// Specifically, the following responsibility is being delegated:
//
// #RFS-A1: If commitIndex > lastApplied: increment lastApplied, apply
// log[lastApplied] to state machine (#5.3)
//
type Committer struct {
	// -- Commit state
	mutex                  sync.Mutex
	cachedIndexOfLastEntry LogIndex
	// commitIndex is the index of highest log entry known to be committed
	// (initialized to 0, increases monotonically)
	cachedCommitIndex      LogIndex
	listeners              map[LogIndex]chan CommandResult // Commit listeners
	highestRegisteredIndex LogIndex

	// -- External components
	log          internal.LogTailOnlyRO
	stateMachine StateMachine
	feHandler    FatalErrorHandler

	// -- Internal components
	commitApplier *util.TriggeredRunner
}

// NewCommitter creates a new Committer with the given parameters.
//
// A goroutine is started that applies committed log entries to the state
// machine. Note that this happens asynchronously to changes in commitIndex.
//
// If the goroutine encounters an error (from the log or the state machine),
// this is fatal for it. In this case, it will call feHandler with the
// encountered error. The feHandler callback is expected to call the
// Committer's StopSync() method before it returns. (or panics)
//
// The value of highestRegisteredIndex will be initialized to commitIndex.
// It will increase when GetResultAsync() is called. When the Log discards
// entries, highestRegisteredIndex will be reset to indexOfLastEntry.
//
func NewCommitter(
	log internal.LogTailOnlyRO,
	commitIndex WatchableIndex,
	stateMachine StateMachine,
	feHandler FatalErrorHandler,
) *Committer {
	// TODO: check that parameters are not nil?!
	// TODO: error if lastApplied > commitIndex!

	c := &Committer{
		listeners:    make(map[LogIndex]chan CommandResult),
		log:          log,
		stateMachine: stateMachine,
		feHandler:    feHandler,
	}

	// Get lock so that we can ensure that initial cached values are correct.
	c.mutex.Lock()
	defer c.mutex.Unlock()

	log.GetIndexOfLastEntryWatchable().AddListener(c.indexOfLastEntryChanged)
	commitIndex.AddListener(c.commitIndexChanged)

	// Get initial cached values after setting up listeners to ensure we
	// don't miss any updates during initialization.
	c.cachedIndexOfLastEntry = log.GetIndexOfLastEntry()
	cachedCommitIndex := commitIndex.Get()
	c.cachedCommitIndex = cachedCommitIndex
	c.highestRegisteredIndex = cachedCommitIndex

	c.commitApplier = util.NewTriggeredRunner(c.applyCommittedEntries)

	return c
}

// StopSync will stop the Committer's goroutine.
//
// Will panic if called more than once.
func (c *Committer) StopSync() {
	// FIXME: should other methods be checking stopped state?
	c.commitApplier.StopSync()
	// FIXME: close pending listeners under lock!?
}

// GetResultAsync asynchronously returns the state machine result for the given
// log index.
//
// When the command at the given log index is applied to the state machine, the
// value returned by the state machine will be sent on the channel by this
// method. If the Log discards the entry at the given log index, the channel is
// closed without a sent value.
//
// The logIndex must be less than or equal to the Log's indexOfLastEntry. This
// means that this method must be called after the log entry has been appended
// to the log.
//
// The logIndex must be greater than commitIndex. This means that this method
// must be called before the consensus algorithm has a chance to advance
// commitIndex.
//
// The logIndex must be greater than highestRegisteredIndex. The logIndex will
// then become the new value of highestRegisteredIndex. This means the
// following:
// - logIndex must generally increase on subsequent calls, except when the Log
// 	discards entries and decreases highestRegisteredIndex.
// - there can be no more than one call to GetResultAsync for a given log
// 	index, except when the Log discards entries.
//
// Note that this method does not have to be called for every log index.
//
func (c *Committer) GetResultAsync(logIndex LogIndex) (<-chan CommandResult, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if logIndex > c.cachedIndexOfLastEntry {
		return nil, fmt.Errorf(
			"FATAL: logIndex=%v is > indexOfLastEntry=%v",
			logIndex, c.cachedIndexOfLastEntry,
		)
	}
	if logIndex <= c.cachedCommitIndex {
		return nil, fmt.Errorf(
			"FATAL: logIndex=%v is <= commitIndex=%v",
			logIndex, c.cachedCommitIndex,
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

func (c *Committer) indexOfLastEntryChanged(oldIole, newIole LogIndex) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if oldIole != c.cachedIndexOfLastEntry {
		return fmt.Errorf(
			"FATAL: oldIole=%v != cachedIndexOfLastEntry=%v",
			oldIole, c.cachedIndexOfLastEntry,
		)
	}

	c.cachedIndexOfLastEntry = newIole

	// If there are registered listeners affected, close their channels
	// and rewind highestRegisteredIndex
	if newIole < c.highestRegisteredIndex {
		for li := newIole + 1; li <= c.highestRegisteredIndex; li++ {
			crc, ok := c.listeners[li]
			if ok {
				delete(c.listeners, li)
				close(crc)
			}
		}
		c.highestRegisteredIndex = newIole
	}

	return nil
}

func (c *Committer) commitIndexChanged(oldCi, newCi LogIndex) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// FIXME: not Committer's responsibility to check this
	if oldCi != c.cachedCommitIndex {
		return fmt.Errorf(
			"FATAL: oldCi=%v != cachedCommitIndex=%v",
			oldCi, c.cachedCommitIndex,
		)
	}
	// FIXME: not Committer's responsibility to check this
	if newCi > c.cachedIndexOfLastEntry {
		return fmt.Errorf(
			"FATAL: commitIndex=%v is > indexOfLastEntry=%v",
			newCi, c.cachedIndexOfLastEntry,
		)
	}
	// FIXME: not Committer's responsibility to check this
	// Check commitIndex is not going backward
	if newCi < oldCi {
		return fmt.Errorf("FATAL: newCi=%v is < oldCi=%v", newCi, oldCi)
	}

	// Update commitIndex and then trigger a run of the applier goroutine
	c.cachedCommitIndex = newCi
	c.commitApplier.TriggerRun()

	return nil
}

func (c *Committer) applyCommittedEntries() {
	// Concurrency:
	// - there is only one method to this call at a time
	// - commitIndex can only increase, so we can snapshot it as a low value

	for {
		// safely get commitIndex
		c.mutex.Lock()
		commitIndexSnapshot := c.cachedCommitIndex
		c.mutex.Unlock()

		lastApplied := c.stateMachine.GetLastApplied()

		// Return if no more entries to apply at this time.
		// (TriggeredRunner should call again if CommitAsync advanced commitIndex)
		// FIXME: error if lastApplied > commitIndex?
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
			crc, haveCrc := c.listeners[indexToApply]
			if haveCrc {
				delete(c.listeners, indexToApply)
			}
			// TODO: since we have the mutex, we could update our copy of commitIndex
			c.mutex.Unlock()

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
