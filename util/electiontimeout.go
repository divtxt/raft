package util

import (
	"math/rand"
	"time"
)

type ElectionTimeoutChooser struct {
	r                  *rand.Rand
	electionTimeoutLow time.Duration
}

func NewElectionTimeoutChooser(electionTimeoutLow time.Duration) *ElectionTimeoutChooser {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return &ElectionTimeoutChooser{r, electionTimeoutLow}
}

func (ret *ElectionTimeoutChooser) ChooseRandomElectionTimeout() time.Duration {
	// #5.2-p6s2: ..., election timeouts are chosen randomly from a fixed
	// interval (e.g., 150-300ms)
	// Currently, we choose a time between electionTimeoutLow and 2*electionTimeoutLow
	timeout := ret.electionTimeoutLow + time.Duration(ret.r.Int63n(int64(ret.electionTimeoutLow)+1))
	return timeout
}
