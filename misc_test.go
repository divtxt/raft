package raft

import (
	"fmt"
	"testing"
	"time"
)

func TestQuorumSizeForClusterSize(t *testing.T) {
	clusterSizes := []uint{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	expectedQrms := []uint{1, 2, 2, 3, 3, 4, 4, 5, 5, 6}

	for i, cs := range clusterSizes {
		if QuorumSizeForClusterSize(cs) != expectedQrms[i] {
			t.Fatal()
		}
	}
}

func TestValidateServerIds(t *testing.T) {
	tests := []struct {
		sid         ServerId
		pids        []ServerId
		expectedErr string
	}{
		{testServerId, testPeerIds, ""},
		{"", testPeerIds, "'thisServerId' is empty string"},
		{testServerId, nil, "'peerServerIds' is nil"},
		{testServerId, []ServerId{}, "'peerServerIds' must have at least one element"},
		{testServerId, []ServerId{"s2", "s2"}, "'peerServerIds' contains duplicate value: s2"},
		{testServerId, []ServerId{"s1", "s2"}, "'peerServerIds' contains 'thisServerId': s1"},
	}

	for _, test := range tests {
		actualErr := ValidateServerIds(test.sid, test.pids)
		if actualErr != test.expectedErr {
			t.Error(fmt.Sprintf("Expected: %v, Actual: %v", test.expectedErr, actualErr))
		}
	}
}

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
