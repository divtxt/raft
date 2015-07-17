package raft

type AppendEntries struct {
	term         TermNo
	prevLogIndex LogIndex
	prevLogTerm  TermNo
}

type AppendEntriesReply struct {
	term    TermNo
	success bool
}
