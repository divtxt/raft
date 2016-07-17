package testdata

import (
	. "github.com/divtxt/raft"
	"time"
)

const (
	ThisServerId = "s1"

	// Note: value for tests based on Figure 7
	// Start as follower at term 7 so that leader will be at term 8
	CurrentTerm = 7

	TickerDuration     = 30 * time.Millisecond
	ElectionTimeoutLow = 150 * time.Millisecond

	SleepToLetGoroutineRun = 10 * time.Millisecond
	SleepJustMoreThanATick = TickerDuration + SleepToLetGoroutineRun

	MaxEntriesPerAppendEntry = 3
)

var AllServerIds = []ServerId{ThisServerId, "s2", "s3", "s4", "s5"}
