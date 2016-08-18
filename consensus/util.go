package consensus

import (
	. "github.com/divtxt/raft"
)

// Helper function
func GetIndexAndTermOfLastEntry(log LogReadOnly) (LogIndex, TermNo, error) {
	lastLogIndex, err := log.GetIndexOfLastEntry()
	if err != nil {
		return 0, 0, err
	}
	var lastLogTerm TermNo = 0
	if lastLogIndex > 0 {
		lastLogTerm, err = log.GetTermAtIndex(lastLogIndex)
		if err != nil {
			return 0, 0, err
		}
	}
	return lastLogIndex, lastLogTerm, nil
}
