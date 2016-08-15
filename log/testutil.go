package log

import (
	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/testdata"
	"strconv"
)

// Make an InMemoryLog with 10 entries with terms as shown in Figure 7, leader line.
func TestUtil_NewInMemoryLog_WithFigure7LeaderLine() *InMemoryLog {
	inmem_log := NewInMemoryLog(10)

	figure7LeaderLine := testdata.TestUtil_MakeFigure7LeaderLineTerms()
	for i, term := range figure7LeaderLine {
		command := Command("c" + strconv.Itoa(i+1))
		inmem_log.AppendEntry(term, command)
	}

	return inmem_log
}
