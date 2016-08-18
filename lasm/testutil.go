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
