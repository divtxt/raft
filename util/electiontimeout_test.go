package util

import (
	"testing"
	"time"
)

func TestElectionTimeoutTracker(t *testing.T) {
	testTickerish := 15 * time.Millisecond
	testElectionTimeoutLow := 150 * time.Millisecond
	testElectionTimeoutHigh := 2 * testElectionTimeoutLow

	now1 := time.Now()
	ett := NewElectionTimeoutTracker(testElectionTimeoutLow, now1)
	if ett.electionTimeoutTime != now1.Add(ett.currentElectionTimeout) {
		t.Fatal()
	}

	timeout1 := ett.currentElectionTimeout
	if timeout1 < testElectionTimeoutLow || timeout1 > testElectionTimeoutHigh {
		t.Fatal(timeout1)
	}

	now2 := now1.Add(testTickerish)
	ett.ChooseNewRandomElectionTimeoutAndTouch(now2)
	timeout2 := ett.currentElectionTimeout
	if timeout2 < testElectionTimeoutLow || timeout2 > testElectionTimeoutHigh {
		t.Fatal(timeout2)
	}

	// Playing the odds here :P
	if timeout1 == timeout2 {
		t.Fatal(timeout1)
	}

	now3 := now2.Add(testTickerish)
	ett.Touch(now3)
	if ett.electionTimeoutTime != now3.Add(ett.currentElectionTimeout) {
		t.Fatal()
	}

	if ett.ElectionTimeoutHasOccurred(now3) {
		t.Fatal()
	}

	now4 := now3.Add(ett.currentElectionTimeout).Add(testTickerish)
	if !ett.ElectionTimeoutHasOccurred(now4) {
		t.Fatal()
	}
}
