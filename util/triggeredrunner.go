package util

import (
	"sync"
)

// A TriggeredRunner is a goroutine that runs a configured function every time it is triggered.
type TriggeredRunner struct {
	f       func()
	trigger chan struct{}
	wg      sync.WaitGroup
}

// NewTriggeredRunner creates a TriggeredRunner for the given function.
//
// The TriggeredRunner's goroutine is started immediately.
func NewTriggeredRunner(f func()) *TriggeredRunner {
	tr := &TriggeredRunner{
		f,
		make(chan struct{}, 1),
		sync.WaitGroup{},
	}
	tr.wg.Add(1)
	go tr.run()
	return tr
}

func (tr *TriggeredRunner) run() {
	defer tr.wg.Done()
	for {
		select {
		case _, ok := <-tr.trigger:
			if !ok {
				return
			}
			tr.f()
		}
	}
}

// TriggerRun triggers a run of the configured function in the TriggeredRunner's goroutine.
//
// This method will return immediately without waiting for the requested function run to start.
//
// If the goroutine is already currently running the function, this trigger will be pending
// and will result in the another run of the function once the current run completes.
// However, multiple such pending triggers will be collapsed into a single pending trigger.
//
// Will panic if StopSync() has been called.
func (tr *TriggeredRunner) TriggerRun() {
	select {
	case tr.trigger <- struct{}{}:
	default: // avoid blocking
	}
}

// StopSync will stop the goroutine, waiting for any current run to complete.
//
// Will panic if called more than once.
func (tr *TriggeredRunner) StopSync() {
	close(tr.trigger)
	tr.wg.Wait()
}

// TestHelperFakeRestart is meant for testing use only.
//
// It resets the state so that TriggerRun() calls will succeed but does not start a new goroutine
// to actually run the function when triggered.
// Use TestHelperRunOnceIfTriggerPending() to actually run the function.
//
// You should have called StopSync() before using this.
//
// See TestTriggeredRunner() for a usage example.
func (tr *TriggeredRunner) TestHelperFakeRestart() {
	tr.trigger = make(chan struct{}, 1)
}

// TestHelperRunOnceIfTriggerPending is meant for testing use only.
//
// It runs the function if a trigger pending, and returns a value indicating if it ran.
//
// You should be using this with TestHelperFakeRestart().
//
// See TestTriggeredRunner() for a usage example.
func (tr *TriggeredRunner) TestHelperRunOnceIfTriggerPending() bool {
	select {
	case <-tr.trigger:
		tr.f()
		return true
	default:
		return false
	}
}
