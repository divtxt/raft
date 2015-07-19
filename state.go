package raft

type PersistentState struct {
	currentTerm TermNo
	log         Log
}
