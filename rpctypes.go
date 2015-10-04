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

	// leader's commitIndex
	leaderCommit LogIndex
}

type AppendEntriesReply struct {
	term    TermNo
	success bool
}

type RpcRequestVote struct {
	// - candidate's term
	term TermNo

	// candidateId

	// - index of candidate's last log entry
	lastLogIndex LogIndex

	// - term of candidate's last log entry
	lastLogTerm TermNo
}

type RpcRequestVoteReply struct {
	// term
	voteGranted bool
}
