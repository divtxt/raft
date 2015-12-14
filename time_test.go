package raft

import (
	"fmt"
	"testing"
	"time"
)

func TestValidateTimeSettings(t *testing.T) {
	tests := []struct {
		timeSettings TimeSettings
		expectedErr  string
	}{
		{
			TimeSettings{5 * time.Millisecond, 50 * time.Millisecond},
			"",
		},
		{
			TimeSettings{0 * time.Millisecond, 50 * time.Millisecond},
			"TickerDuration must be greater than zero",
		},
		{
			TimeSettings{-1 * time.Millisecond, 50 * time.Millisecond},
			"TickerDuration must be greater than zero",
		},
		{
			TimeSettings{2 * time.Millisecond, 1 * time.Millisecond},
			"ElectionTimeoutLow must be greater than TickerDuration",
		},
		{
			TimeSettings{1 * time.Millisecond, -2 * time.Millisecond},
			"ElectionTimeoutLow must be greater than TickerDuration",
		},
	}

	for _, test := range tests {
		actualErr := ValidateTimeSettings(test.timeSettings)
		if actualErr != test.expectedErr {
			t.Error(fmt.Sprintf("Expected: %v, Actual: %v", test.expectedErr, actualErr))
		}
	}
}

func TestElectionTimeoutTracker(t *testing.T) {
	testTickerish := 15 * time.Millisecond
	testElectionTimeoutLow := 150 * time.Millisecond
	testElectionTimeoutHigh := 2 * testElectionTimeoutLow

	now1 := time.Now()
	ett := newElectionTimeoutTracker(testElectionTimeoutLow, now1)
	if ett.electionTimeoutTime != now1.Add(ett.currentElectionTimeout) {
		t.Fatal()
	}

	timeout1 := ett.currentElectionTimeout
	if timeout1 < testElectionTimeoutLow || timeout1 > testElectionTimeoutHigh {
		t.Fatal(timeout1)
	}

	ett.chooseNewRandomElectionTimeout()
	timeout2 := ett.currentElectionTimeout
	if timeout2 < testElectionTimeoutLow || timeout2 > testElectionTimeoutHigh {
		t.Fatal(timeout2)
	}

	// Playing the odds here :P
	if timeout1 == timeout2 {
		t.Fatal(timeout1)
	}

	now2 := now1.Add(testTickerish)
	ett.resetElectionTimeoutTime(now2)
	if ett.electionTimeoutTime != now2.Add(ett.currentElectionTimeout) {
		t.Fatal()
	}

	if ett.electionTimeoutHasOccurred(now2) {
		t.Fatal()
	}

	now3 := now2.Add(ett.currentElectionTimeout).Add(testTickerish)
	if !ett.electionTimeoutHasOccurred(now3) {
		t.Fatal()
	}
}
