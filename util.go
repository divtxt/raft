package raft

// Helper function
func GetIndexAndTermOfLastEntry(log Log) (LogIndex, TermNo) {
	lastLogIndex, err := log.GetIndexOfLastEntry()
	if err != nil {
		panic(err)
	}
	var lastLogTerm TermNo = 0
	if lastLogIndex > 0 {
		lastLogTerm, err = log.GetTermAtIndex(lastLogIndex)
		if err != nil {
			panic(err)
		}
	}
	return lastLogIndex, lastLogTerm
}
