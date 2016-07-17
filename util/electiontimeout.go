package util

import (
	"math/rand"
	"time"
)

type ElectionTimeoutTracker struct {
	r                      *rand.Rand
	electionTimeoutLow     time.Duration
	currentElectionTimeout time.Duration
	electionTimeoutTime    time.Time
}

func NewElectionTimeoutTracker(
	electionTimeoutLow time.Duration,
	now time.Time,
) *ElectionTimeoutTracker {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	ett := &ElectionTimeoutTracker{
		r,
		electionTimeoutLow,
		0,   // temp value, to be replaced later in initialization
		now, // temp value, to be replaced before goroutine start
	}

	ett.ChooseNewRandomElectionTimeoutAndTouch(now)

	return ett
}

func (ett *ElectionTimeoutTracker) ChooseNewRandomElectionTimeoutAndTouch(now time.Time) {
	// #5.2-p6s2: ..., election timeouts are chosen randomly from a fixed
	// interval (e.g., 150-300ms)
	ett.currentElectionTimeout = ett.electionTimeoutLow + time.Duration(ett.r.Int63n(int64(ett.electionTimeoutLow)+1))
	ett.Touch(now)
}

func (ett *ElectionTimeoutTracker) Touch(now time.Time) {
	ett.electionTimeoutTime = now.Add(ett.currentElectionTimeout)
}

func (ett *ElectionTimeoutTracker) ElectionTimeoutHasOccurred(now time.Time) bool {
	return now.After(ett.electionTimeoutTime)
}

func (ett *ElectionTimeoutTracker) GetCurrentElectionTimeout() time.Duration {
	return ett.currentElectionTimeout
}

func (ett *ElectionTimeoutTracker) GetElectionTimeoutTime() time.Time {
	return ett.electionTimeoutTime
}
