package applier

import (
	"fmt"
	"sync"

	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/internal"
	"github.com/divtxt/raft/util"
)

type FatalErrorHandler func(err error)

// An Applier is a goroutine that applies committed log entries to the state
// machine and notifies the result listener for each entry.
//
// Specifically, the following responsibility is being delegated:
//
// #RFS-A1: If commitIndex > lastApplied: increment lastApplied, apply
// log[lastApplied] to state machine (#5.3)
//
type Applier struct {
	// -- State
	mutex                  sync.Mutex
	cachedIndexOfLastEntry LogIndex
	// commitIndex is the index of highest log entry known to be committed
	// (initialized to 0, increases monotonically)
	cachedCommitIndex      LogIndex
	listeners              map[LogIndex]chan CommandResult // Result listeners
	highestRegisteredIndex LogIndex

	// -- External components
	log          internal.LogTailRO
	stateMachine StateMachine
	feHandler    FatalErrorHandler

	// -- Internal components
	runner *util.TriggeredRunner
}

// NewApplier creates a new Applier with the given parameters.
//
// A goroutine is started that applies committed log entries to the state
// machine. Note that this happens asynchronously to changes in commitIndex.
//
// If the goroutine encounters an error (from the log or the state machine),
// this is fatal for it. In this case, it will call feHandler with the
// encountered error. The feHandler callback is expected to call the
// Applier's StopSync() method before it returns. (or panics)
// FIXME: why can't it stop itself?!
//
// The value of highestRegisteredIndex will be initialized to commitIndex.
// It will increase when GetResultAsync() is called. When the Log discards
// entries, highestRegisteredIndex will be reset to indexOfLastEntry.
//
func NewApplier(
	log internal.LogTailRO,
	commitIndex WatchableIndex,
	stateMachine StateMachine,
	feHandler FatalErrorHandler,
) *Applier {
	// TODO: check that parameters are not nil?!
	// TODO: error if lastApplied > commitIndex!

	a := &Applier{
		listeners:    make(map[LogIndex]chan CommandResult),
		log:          log,
		stateMachine: stateMachine,
		feHandler:    feHandler,
	}

	// Get lock so that we can ensure that initial cached values are correct.
	a.mutex.Lock()
	defer a.mutex.Unlock()

	log.GetIndexOfLastEntryWatchable().AddListener(a.indexOfLastEntryChanged)
	commitIndex.AddListener(a.commitIndexChanged)

	// Get initial cached values after setting up listeners to ensure we
	// don't miss any updates during initialization.
	a.cachedIndexOfLastEntry = log.GetIndexOfLastEntry()
	cachedCommitIndex := commitIndex.Get()
	a.cachedCommitIndex = cachedCommitIndex
	a.highestRegisteredIndex = cachedCommitIndex

	a.runner = util.NewTriggeredRunner(a.applyCommittedEntries)

	return a
}

// StopSync will stop the Applier's goroutine.
//
// Will panic if called more than once.
func (a *Applier) StopSync() {
	// FIXME: should other methods be checking stopped state?
	a.runner.StopSync()
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
func (a *Applier) GetResultAsync(logIndex LogIndex) (<-chan CommandResult, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if logIndex > a.cachedIndexOfLastEntry {
		return nil, fmt.Errorf(
			"FATAL: logIndex=%v is > indexOfLastEntry=%v",
			logIndex, a.cachedIndexOfLastEntry,
		)
	}
	if logIndex <= a.cachedCommitIndex {
		return nil, fmt.Errorf(
			"FATAL: logIndex=%v is <= commitIndex=%v",
			logIndex, a.cachedCommitIndex,
		)
	}
	if logIndex <= a.highestRegisteredIndex {
		return nil, fmt.Errorf(
			"FATAL: logIndex=%v is <= highestRegisteredIndex=%v",
			logIndex,
			a.highestRegisteredIndex,
		)
	}

	crc := make(chan CommandResult, 1)

	a.listeners[logIndex] = crc
	a.highestRegisteredIndex = logIndex

	return crc, nil
}

func (a *Applier) indexOfLastEntryChanged(oldIole, newIole LogIndex) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if oldIole != a.cachedIndexOfLastEntry {
		return fmt.Errorf(
			"FATAL: oldIole=%v != cachedIndexOfLastEntry=%v",
			oldIole, a.cachedIndexOfLastEntry,
		)
	}

	a.cachedIndexOfLastEntry = newIole

	// If there are registered listeners affected, close their channels
	// and rewind highestRegisteredIndex
	if newIole < a.highestRegisteredIndex {
		for li := newIole + 1; li <= a.highestRegisteredIndex; li++ {
			crc, ok := a.listeners[li]
			if ok {
				delete(a.listeners, li)
				close(crc)
			}
		}
		a.highestRegisteredIndex = newIole
	}

	return nil
}

func (a *Applier) commitIndexChanged(oldCi, newCi LogIndex) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// FIXME: not Applier's responsibility to check this
	if oldCi != a.cachedCommitIndex {
		return fmt.Errorf(
			"FATAL: oldCi=%v != cachedCommitIndex=%v",
			oldCi, a.cachedCommitIndex,
		)
	}
	// FIXME: not Applier's responsibility to check this
	if newCi > a.cachedIndexOfLastEntry {
		return fmt.Errorf(
			"FATAL: commitIndex=%v is > indexOfLastEntry=%v",
			newCi, a.cachedIndexOfLastEntry,
		)
	}
	// FIXME: not Applier's responsibility to check this
	// Check commitIndex is not going backward
	if newCi < oldCi {
		return fmt.Errorf("FATAL: newCi=%v is < oldCi=%v", newCi, oldCi)
	}

	// Update commitIndex and then trigger a run of the applier goroutine
	a.cachedCommitIndex = newCi
	a.runner.TriggerRun()

	return nil
}

func (a *Applier) applyCommittedEntries() {
	// Concurrency:
	// - there is only one method to this call at a time
	// - commitIndex can only increase, so we can snapshot it as a low value

	for {
		// safely get commitIndex
		a.mutex.Lock()
		commitIndexSnapshot := a.cachedCommitIndex
		a.mutex.Unlock()

		lastApplied := a.stateMachine.GetLastApplied()

		// Return if no more entries to apply at this time.
		// (TriggeredRunner should call again if CommitAsync advanced commitIndex)
		// FIXME: error if lastApplied > commitIndex?
		if lastApplied >= commitIndexSnapshot {
			return
		}

		// Get a batch of entries from the raft log.
		entries, err := a.log.GetEntriesAfterIndex(lastApplied)
		if err != nil {
			a.feHandler(err)
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
			a.mutex.Lock()
			crc, haveCrc := a.listeners[indexToApply]
			if haveCrc {
				delete(a.listeners, indexToApply)
			}
			// TODO: since we have the mutex, we could update our copy of commitIndex
			a.mutex.Unlock()

			// Apply the command to the state machine.
			commandResult := a.stateMachine.ApplyCommand(indexToApply, entry.Command)

			// Send the result to the commit listener.
			if haveCrc {
				crc <- commandResult
			}

			// The index of the entry we have just applied MUST be the new value of lastApplied.
			lastApplied = indexToApply
		}
	}
}
