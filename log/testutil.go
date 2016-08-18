package log

import (
	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/testdata"
	"strconv"
)

// Make an InMemoryLog with 10 entries with terms as shown in Figure 7, leader line.
// Commands will be Command("c1"), Command("c2"), etc.
func TestUtil_NewInMemoryLog_WithFigure7LeaderLine(maxEntriesPerAppendEntry uint64) *InMemoryLog {
	figure7LeaderLine := testdata.TestUtil_MakeFigure7LeaderLineTerms()
	return TestUtil_NewInMemoryLog_WithTerms(figure7LeaderLine, maxEntriesPerAppendEntry)
}

// Make an InMemoryLog with entries with given terms.
// Commands will be Command("c1"), Command("c2"), etc.
func TestUtil_NewInMemoryLog_WithTerms(
	logTerms []TermNo,
	maxEntriesPerAppendEntry uint64,
) *InMemoryLog {
	inmem_log := NewInMemoryLog(maxEntriesPerAppendEntry)

	for i, term := range logTerms {
		command := Command("c" + strconv.Itoa(i+1))
		inmem_log.AppendEntry(term, command)
	}

	return inmem_log
}
