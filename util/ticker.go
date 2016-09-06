package util

import (
	"sync"
	"time"
)

type Ticker struct {
	f            func()
	d            time.Duration
	ticker       *time.Ticker
	stopSignal   chan struct{}
	runTicksDone *sync.WaitGroup
}

// Create a Ticker that calls the given function at the given interval.
//
// A goroutine is started that will call the given function.
// Drops ticks for slow receivers.
func NewTicker(f func(), d time.Duration) *Ticker {
	t := &Ticker{
		f:            f,
		d:            d,
		ticker:       time.NewTicker(d),
		stopSignal:   make(chan struct{}, 1),
		runTicksDone: &sync.WaitGroup{},
	}

	t.runTicksDone.Add(1)
	go t.runTicks()

	return t
}

// Stop the ticker and wait for the goroutine to finish.
// This means it will wait till a running function is done.
// Should only be called once.
func (t *Ticker) StopSync() {
	close(t.stopSignal)
	t.runTicksDone.Wait()
}

func (t *Ticker) runTicks() {
	defer t.runTicksDone.Done()
	defer t.ticker.Stop()

	for {
		select {
		case <-t.ticker.C:
			t.f()
		case <-t.stopSignal:
			return
		}
	}
}
