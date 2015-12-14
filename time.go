package raft

import (
	"math/rand"
	"time"
)

type TimeSettings struct {
	TickerDuration time.Duration

	// Election timeout low value - 2x this value is used as high value.
	ElectionTimeoutLow time.Duration
}

// Check values of a TimeSettings value:
//
//    tickerDuration  must be greater than zero.
//    electionTimeout must be greater than tickerDuration.
//
// These are just basic sanity checks and currently don't include the
// softer usefulness checks recommended by the raft protocol.
func ValidateTimeSettings(timeSettings TimeSettings) string {
	if timeSettings.TickerDuration.Nanoseconds() <= 0 {
		return "TickerDuration must be greater than zero"
	}
	if timeSettings.ElectionTimeoutLow.Nanoseconds() <= timeSettings.TickerDuration.Nanoseconds() {
		return "ElectionTimeoutLow must be greater than TickerDuration"
	}

	return ""
}

type electionTimeoutTracker struct {
	electionTimeoutLow     time.Duration
	currentElectionTimeout time.Duration
	electionTimeoutTime    time.Time
}

func newElectionTimeoutTracker(electionTimeoutLow time.Duration, now time.Time) *electionTimeoutTracker {
	ett := &electionTimeoutTracker{
		electionTimeoutLow,
		0,   // temp value, to be replaced later in initialization
		now, // temp value, to be replaced before goroutine start
	}

	ett.chooseNewRandomElectionTimeout()
	ett.resetElectionTimeoutTime(now)

	return ett
}

func (ett *electionTimeoutTracker) chooseNewRandomElectionTimeout() {
	// #5.2-p6s2: ..., election timeouts are chosen randomly from a fixed
	// interval (e.g., 150-300ms)
	ett.currentElectionTimeout = ett.electionTimeoutLow + time.Duration(rand.Int63n(int64(ett.electionTimeoutLow)+1))
}

func (ett *electionTimeoutTracker) resetElectionTimeoutTime(now time.Time) {
	ett.electionTimeoutTime = now.Add(ett.currentElectionTimeout)
}

func (ett *electionTimeoutTracker) electionTimeoutHasOccurred(now time.Time) bool {
	return now.After(ett.electionTimeoutTime)
}
