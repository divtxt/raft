package util

type TriggeredRunner struct {
	*StoppableGoroutine
	f func()
}

// NewTriggeredRunner starts a goroutine that runs the given function every time it is triggered.
//
// It uses StoppableGoroutine so that the underlying goroutine can be stopped.
//
func NewTriggeredRunner(f func()) *TriggeredRunner {
	tr := &TriggeredRunner{
		nil,
		f,
	}

	tr.StoppableGoroutine = StartGoroutine(tr.run)

	return tr
}

func (tr *TriggeredRunner) run(stop <-chan struct{}) {
	for {
		select {
		// Use stop channel as trigger channel so that pending runs don't lose to channel close.
		case _, ok := <-stop:
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
// Will panic if the underlying StoppableGoroutine's StopAsync or StopSync has been called.
//
func (tr *TriggeredRunner) TriggerRun() {
	select {
	case tr.stop <- struct{}{}:
	default: // avoid blocking
	}
}

// TestHelperFakeRestart is meant for testing use only.
//
// It resets the state so that TriggerRun() calls will succeed but does not start a new goroutine
// to actually run the function when triggered.
// Use TestHelperRunOnceIfTriggerPending() to actually run the function if a trigger is pending.
//
// You should have called StopSync() before using this.
//
// See TestTriggeredRunner() for a usage example.
func (tr *TriggeredRunner) TestHelperFakeRestart() {
	tr.stop = make(chan struct{}, 1)
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
	case <-tr.stop:
		tr.f()
		return true
	default:
		return false
	}
}
