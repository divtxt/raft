package util

type StoppableFunc func(stop <-chan struct{})

type StoppableGoroutine struct {
	stop    chan struct{}
	stopped chan struct{}
}

// StartGoroutine starts a new goroutine that calls the given function.
//
// The function is generally expected to be long-running and able to stop when requested.
// The stop request is signaled by closing the channel sent to the function.
//
func StartGoroutine(f StoppableFunc) *StoppableGoroutine {
	sg := &StoppableGoroutine{
		stop:    make(chan struct{}, 1),
		stopped: make(chan struct{}, 1),
	}

	go func() {
		defer close(sg.stopped)
		f(sg.stop)
	}()

	return sg
}

// StopAsync sends an asynchronous request to stop to the underlying function in the goroutine.
//
// The request is indicated by closing the channel passed to the function.
// The function can take a while to stop or ignore the request.
//
// Safe to call once even if the goroutine has already finished.
// Will panic if called more than once.
func (sg *StoppableGoroutine) StopAsync() {
	close(sg.stop)
}

// Wait for the goroutine to finish.
//
// Returns immediately if the goroutine has finished.
func (sg *StoppableGoroutine) Join() {
	<-sg.stopped
}

// StopSync stops the goroutine and waits for it to finish.
//
// Identical to calling StopAsync() and then Join().
func (sg *StoppableGoroutine) StopSync() {
	sg.StopAsync()
	sg.Join()
}

// Stopped checks if the goroutine has finished running.
//
// This will only be true after both the Stoppable and the resultHandler have finished.
func (sg *StoppableGoroutine) Stopped() bool {
	select {
	case <-sg.stopped:
		return true
	default:
		return false
	}
}
