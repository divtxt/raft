// The Raft paper models communication between consensus modules as
// concurrent RPC method calls that return values.
// However, we're implementing this consensus module as a single threaded
// component that expects an asynchronous messaging model implementation.
// In this model, RPC requests and replies are independent message types
// that are sent and received using the same interfaces.
// The actual choice of networking protocol, delivery semantics and policy
// are intentionally unspecified by this implementation.

package raft

type AppendEntries struct {
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
	LeaderCommit LogIndex
}

type AppendEntriesReply struct {
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
