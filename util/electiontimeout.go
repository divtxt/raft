package util

import (
	"math/rand"
	"time"
)

var r *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

type ElectionTimeoutChooser struct {
	electionTimeoutLow time.Duration
}

func NewElectionTimeoutChooser(electionTimeoutLow time.Duration) *ElectionTimeoutChooser {
	return &ElectionTimeoutChooser{electionTimeoutLow}
}

func (etc *ElectionTimeoutChooser) ChooseRandomElectionTimeout() time.Duration {
	// #5.2-p6s2: ..., election timeouts are chosen randomly from a fixed
	// interval (e.g., 150-300ms)
	// Currently, we choose a time between electionTimeoutLow and 2*electionTimeoutLow
	timeout := etc.electionTimeoutLow + time.Duration(r.Int63n(int64(etc.electionTimeoutLow)+1))
	return timeout
}
