package config_test

import (
	"fmt"
	"github.com/divtxt/raft/config"
	"testing"
	"time"
)

func TestValidateTimeSettings(t *testing.T) {
	tests := []struct {
		timeSettings config.TimeSettings
		expectedErr  string
	}{
		{
			config.TimeSettings{5 * time.Millisecond, 50 * time.Millisecond},
			"",
		},
		{
			config.TimeSettings{0 * time.Millisecond, 50 * time.Millisecond},
			"TickerDuration must be greater than zero",
		},
		{
			config.TimeSettings{-1 * time.Millisecond, 50 * time.Millisecond},
			"TickerDuration must be greater than zero",
		},
		{
			config.TimeSettings{2 * time.Millisecond, 1 * time.Millisecond},
			"ElectionTimeoutLow must be greater than TickerDuration",
		},
		{
			config.TimeSettings{1 * time.Millisecond, -2 * time.Millisecond},
			"ElectionTimeoutLow must be greater than TickerDuration",
		},
	}

	for _, test := range tests {
		actualErr := config.ValidateTimeSettings(test.timeSettings)
		if actualErr != test.expectedErr {
			t.Error(fmt.Sprintf("Expected: %v, Actual: %v", test.expectedErr, actualErr))
		}
	}
}
