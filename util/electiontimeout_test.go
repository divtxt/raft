package util

import (
	"testing"
	"time"
)

func TestElectionTimeoutChooser(t *testing.T) {
	testElectionTimeoutLow := 150 * time.Millisecond
	testElectionTimeoutHigh := 2 * testElectionTimeoutLow

	etc := NewElectionTimeoutChooser(testElectionTimeoutLow)

	timeout1 := etc.ChooseRandomElectionTimeout()
	if timeout1 < testElectionTimeoutLow || timeout1 > testElectionTimeoutHigh {
		t.Fatal(timeout1)
	}

	timeout2 := etc.ChooseRandomElectionTimeout()
	if timeout2 < testElectionTimeoutLow || timeout2 > testElectionTimeoutHigh {
		t.Fatal(timeout2)
	}

	// Playing the odds here :P
	if timeout1 == timeout2 {
		t.Fatal(timeout1)
	}
}
