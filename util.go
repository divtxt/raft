package raft

// Helper function
func GetIndexAndTermOfLastEntry(log Log) (LogIndex, TermNo) {
	lastLogIndex := log.GetIndexOfLastEntry()
	var lastLogTerm TermNo = 0
	if lastLogIndex > 0 {
		lastLogTerm = log.GetTermAtIndex(lastLogIndex)
	}
	return lastLogIndex, lastLogTerm
}
