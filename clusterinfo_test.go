package raft

import (
	"testing"
)

func TestNewClusterInfo_Validation(t *testing.T) {
	tests := []struct {
		aids        []ServerId
		tid         ServerId
		expectedErr string
	}{
		{
			nil,
			"s1",
			"allServerIds is nil",
		},
		{
			[]ServerId{"s1"},
			"s1",
			"allServerIds must have at least 2 elements",
		},
		{
			[]ServerId{"s1", "s2"},
			"",
			"thisServerId is empty string",
		},
		{
			[]ServerId{"s1", ""},
			"s1",
			"allServerIds contains empty string",
		},
		{
			[]ServerId{"s1", "s2", "s2"},
			"s1",
			"allServerIds contains duplicate value: s2",
		},
		{
			[]ServerId{"s2", "s3"},
			"s1",
			"allServerIds does not contain thisServerId: s1",
		},
	}

	for _, test := range tests {
		test_ExpectPanic(
			t,
			func() {
				NewClusterInfo(test.aids, test.tid)
			},
			test.expectedErr,
		)
	}
}

func TestQuorumSizeForClusterSize(t *testing.T) {
	clusterSizes := []uint{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	expectedQrms := []uint{1, 2, 2, 3, 3, 4, 4, 5, 5, 6}

	for i, cs := range clusterSizes {
		if QuorumSizeForClusterSize(cs) != expectedQrms[i] {
			t.Fatal()
		}
	}
}
