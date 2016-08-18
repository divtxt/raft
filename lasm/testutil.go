package lasm

import (
	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/log"
	"github.com/divtxt/raft/testhelpers"
	"strconv"
)

// Test Helper
func TestUtil_NewLasmiWithDummyCommands(
	logTerms []TermNo,
	maxEntriesPerAppendEntry uint64,
) *LogAndStateMachineImpl {

	iml := log.NewInMemoryLog(maxEntriesPerAppendEntry)
	for i, term := range logTerms {
		command := Command("c" + strconv.Itoa(i+1))
		iml.AppendEntry(term, command)
	}

	dsm := testhelpers.NewDummyStateMachine()

	return NewLogAndStateMachineImpl(iml, dsm)
}

// Test Helper
func TestHelper_GetLogEntryAtIndex(lasm LogAndStateMachine, li LogIndex) LogEntry {
	if li == 0 {
		panic("oops!")
	}
	entries, err := lasm.GetEntriesAfterIndex(li - 1)
	if err != nil {
		panic(err)
	}
	return entries[0]
}
