package raft

import (
	"testing"
)

func TestLeaderVolatileState(t *testing.T) {
	ci := NewClusterInfo(testAllServerIds, testThisServerId)
	newLeaderVolatileState(ci)
}
