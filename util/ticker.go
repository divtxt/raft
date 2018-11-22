package util

import (
	"time"
)

type Ticker struct {
	*StoppableGoroutine
	f      func()
	ticker *time.Ticker
}

// NewTicker starts a goroutine that calls the given function at the given interval.
//
// Drops ticks for slow receivers.
//
// Ticker uses StoppableGoroutine to support stopping the goroutine.
//
func NewTicker(f func(), d time.Duration) *Ticker {
	t := &Ticker{
		f:      f,
		ticker: time.NewTicker(d),
	}

	t.StoppableGoroutine = StartGoroutine(t.runTicks)

	return t
}

func (t *Ticker) runTicks(stop <-chan struct{}) {
	defer t.ticker.Stop()

	for {
		select {
		case <-t.ticker.C:
			t.f()
		case <-stop:
			return
		}
	}
}
