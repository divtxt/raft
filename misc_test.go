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
