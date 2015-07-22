package raft

type AppendEntries struct {
	// - leader's term
	term TermNo

	// leaderId

	// - index of log entry immediately preceding new ones
	prevLogIndex LogIndex

	// - term of prevLogIndex entry
	prevLogTerm TermNo

	// - log entries to store (empty for heartbeat; may send more than one ...)
	// implied?: terms in these entries >= prevLogTerm
	// implied?: term in each new entry >= term of previous entry
	entries []LogEntry

	// leaderCommit
}

type AppendEntriesReply struct {
	term    TermNo
	success bool
}
