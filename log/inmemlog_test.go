package log

import (
	"testing"
)

// Test InMemoryLog using the Log Blackbox test.
func TestInMemoryLog_BlackboxTest(t *testing.T) {
	inmem_log := TestUtil_NewInMemoryLog_WithFigure7LeaderLine()

	BlackboxTest_Log(t, inmem_log)
}
