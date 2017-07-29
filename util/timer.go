package util

import (
	"time"
)

type Timer struct {
	nowFunc   func() time.Time
	duration  time.Duration
	expiresAt time.Time
}

func NewTimer(duration time.Duration, nowFunc func() time.Time) *Timer {
	timer := &Timer{
		nowFunc,
		duration,
		nowFunc(), // temp value
	}
	timer.Restart()
	return timer
}

// Restart the timer with the given duration.
func (t *Timer) RestartWithDuration(duration time.Duration) {
	t.duration = duration
	t.Restart()
}

// Restart the timer with the current duration.
func (t *Timer) Restart() {
	t.expiresAt = t.nowFunc().Add(t.duration)
}

// Check if the timer duration has expired.
func (t *Timer) Expired() bool {
	return t.nowFunc().After(t.expiresAt)
}

// Get the current timer duration.
func (t *Timer) GetCurrentDuration() time.Duration {
	return t.duration
}

func (t *Timer) GetExpiryTime() time.Time {
	return t.expiresAt
}
