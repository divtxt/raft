package raft

import (
	"testing"
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
