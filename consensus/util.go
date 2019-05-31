package consensus

import (
	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/internal"
)

// Helper function
func GetIndexAndTermOfLastEntry(log internal.LogTailOnlyRO) (LogIndex, TermNo, error) {
	lastLogIndex := log.GetIndexOfLastEntry()
	var lastLogTerm TermNo = 0
	if lastLogIndex > 0 {
		var err error
		lastLogTerm, err = log.GetTermAtIndex(lastLogIndex)
		if err != nil {
			return 0, 0, err
		}
	}
	return lastLogIndex, lastLogTerm, nil
}
