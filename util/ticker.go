package util

import (
	"time"
)

type Ticker struct {
	f          func()
	d          time.Duration
	ticker     *time.Ticker
	stopSignal chan struct{}
}

// Create a Ticker that calls the given function at the given interval.
//
// A goroutine is started that will call the given function.
// Drops ticks for slow receivers.
func NewTicker(f func(), d time.Duration) *Ticker {
	t := &Ticker{
		f:          f,
		d:          d,
		ticker:     time.NewTicker(d),
		stopSignal: make(chan struct{}, 1),
	}

	go t.runTicks()

	return t
}

// Stop the ticker asynchronously.
//
// Should only be called once.
func (t *Ticker) StopAsync() {
	close(t.stopSignal)
}

func (t *Ticker) runTicks() {
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
