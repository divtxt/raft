// See RpcService in interfaces.go for related details.

package raft

type RpcAppendEntries struct {
	// - leader's term
	Term TermNo

	// leaderId

	// - index of log entry immediately preceding new ones
	PrevLogIndex LogIndex

	// - term of prevLogIndex entry
	PrevLogTerm TermNo

	// - log entries to store (empty for heartbeat; may send more than one ...)
	// implied?: terms in these entries >= prevLogTerm
	// implied?: term in each new entry >= term of previous entry
	Entries []LogEntry

	// leader's commitIndex
	// implied?: LeaderCommit >= PrevLogIndex
	LeaderCommit LogIndex
}

type RpcAppendEntriesReply struct {
	Term    TermNo
	Success bool
}

type RpcRequestVote struct {
	// - candidate's term
	Term TermNo

	// candidateId

	// - index of candidate's last log entry
	LastLogIndex LogIndex

	// - term of candidate's last log entry
	LastLogTerm TermNo
}

type RpcRequestVoteReply struct {
	// term
	VoteGranted bool
}
