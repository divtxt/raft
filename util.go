package raft

// Helper function
func GetIndexAndTermOfLastEntry(lasm LogAndStateMachine) (LogIndex, TermNo, error) {
	lastLogIndex, err := lasm.GetIndexOfLastEntry()
	if err != nil {
		return 0, 0, err
	}
	var lastLogTerm TermNo = 0
	if lastLogIndex > 0 {
		lastLogTerm, err = lasm.GetTermAtIndex(lastLogIndex)
		if err != nil {
			return 0, 0, err
		}
	}
	return lastLogIndex, lastLogTerm, nil
}
