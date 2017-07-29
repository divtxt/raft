package util

import (
	"testing"
	"time"
)

func TestTimerMockTime(t *testing.T) {
	now := time.Now()
	nowFunc := func() time.Time { return now }
	sleepFunc := func(d time.Duration) { now = now.Add(d) }
	timerImplTests(
		t,
		nowFunc,
		sleepFunc,
		150*time.Millisecond,
	)
}

func TestTimerRealTime(t *testing.T) {
	timerImplTests(
		t,
		time.Now,
		time.Sleep,
		10*time.Millisecond,
	)
}

func timerImplTests(
	t *testing.T,
	nowFunc func() time.Time,
	sleepFunc func(time.Duration),
	testTick time.Duration,
) {
	timer := NewTimer(3*testTick, nowFunc)
	if timer.Expired() {
		t.Fatal()
	}

	if timer.GetCurrentDuration() != 3*testTick {
		t.Fatal()
	}

	// Basic sleep check
	sleepFunc(2 * testTick)
	if timer.Expired() {
		t.Fatal()
	}

	// Keep pushing expiry out
	timer.Restart()
	if timer.GetCurrentDuration() != 3*testTick {
		t.Fatal()
	}
	sleepFunc(2 * testTick)
	if timer.Expired() {
		t.Fatal()
	}
	timer.Restart()
	sleepFunc(2 * testTick)
	if timer.Expired() {
		t.Fatal()
	}

	// Change duration
	timer.RestartWithDuration(testTick)
	if timer.GetCurrentDuration() != testTick {
		t.Fatal()
	}
	sleepFunc(2 * testTick)
	if !timer.Expired() {
		t.Fatal()
	}

	// Restart after expiry
	timer.Restart()
	if timer.Expired() {
		t.Fatal()
	}
	timer.RestartWithDuration(2 * testTick)
	if timer.GetCurrentDuration() != 2*testTick {
		t.Fatal()
	}
	sleepFunc(testTick)
	if timer.Expired() {
		t.Fatal()
	}
	sleepFunc(2 * testTick)
	if !timer.Expired() {
		t.Fatal()
	}
}
