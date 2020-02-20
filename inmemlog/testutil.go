package inmemlog

import (
	"strconv"

	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/testdata"
)

// Make an InMemoryLog with 10 entries with terms as shown in Figure 7, leader line.
// Commands will be Command("c1"), Command("c2"), etc.
func TestUtil_NewInMemoryLog_WithFigure7LeaderLine(maxEntries uint64) (*InMemoryLog, error) {
	figure7LeaderLine := testdata.TestUtil_MakeFigure7LeaderLineTerms()
	return TestUtil_NewInMemoryLog_WithTerms(figure7LeaderLine, maxEntries)
}

// Make an InMemoryLog with entries with given terms.
// Commands will be Command("c1"), Command("c2"), etc.
func TestUtil_NewInMemoryLog_WithTerms(
	logTerms []TermNo, maxEntries uint64,
) (*InMemoryLog, error) {
	inmem_log, err := NewInMemoryLog(maxEntries)
	if err != nil {
		return nil, err
	}

	for i, term := range logTerms {
		command := Command("c" + strconv.Itoa(i+1))
		logEntry := LogEntry{term, command}
		_, err := inmem_log.AppendEntry(logEntry)
		if err != nil {
			panic(err)
		}
	}

	return inmem_log, nil
}
