package log

import (
	. "github.com/divtxt/raft"
	"strconv"
	"testing"
)

// Blackbox test
// Send a Log with 10 entries with terms as shown in Figure 7, leader line.
// No commands should have been applied yet.
func TestInMemoryLog_Blackbox(t *testing.T) {
	figure7LeaderLine := BlackboxTest_MakeFigure7LeaderLineTerms()

	inmem_log := NewInMemoryLog(10)

	for i, term := range figure7LeaderLine {
		command := Command("c" + strconv.Itoa(i+1))
		inmem_log.AppendEntry(term, command)
	}

	BlackboxTest_Log(t, inmem_log)
}
