package raft

import (
	"fmt"
	"reflect"
	"testing"
)

// Extra: ignore replies for previous term rpc
func TestCM_RpcAER_All_IgnorePreviousTermRpc(t *testing.T) {
	f := func(setup func(t *testing.T) (mcm *managedConsensusModule, mrs *mockRpcSender)) {
		mcm, mrs := setup(t)
		serverTerm := mcm.pcm.persistentState.GetCurrentTerm()
		sentRpc := makeAEWithTerm(serverTerm - 1)
		beforeState := mcm.pcm.getServerState()

		err := mcm.pcm.rpcReply_RpcAppendEntriesReply(
			"s2",
			sentRpc,
			&RpcAppendEntriesReply{serverTerm, true},
		)
		if err != nil {
			t.Fatal(err)
		}
		if mcm.pcm.getServerState() != beforeState {
			t.Fatal()
		}
		if mcm.pcm.persistentState.GetCurrentTerm() != serverTerm {
			t.Fatal()
		}
		if beforeState == LEADER {
			expectedNextIndex := map[ServerId]LogIndex{"s2": 11, "s3": 11, "s4": 11, "s5": 11}
			if !reflect.DeepEqual(mcm.pcm.leaderVolatileState.nextIndex, expectedNextIndex) {
				t.Fatal()
			}
			expectedMatchIndex := map[ServerId]LogIndex{"s2": 0, "s3": 0, "s4": 0, "s5": 0}
			if !reflect.DeepEqual(mcm.pcm.leaderVolatileState.matchIndex, expectedMatchIndex) {
				t.Fatal()
			}
			mrs.checkSentRpcs(t, []mockSentRpc{})
		}
	}

	f(testSetupMCM_Follower_Figure7LeaderLine)
	f(testSetupMCM_Candidate_Figure7LeaderLine)
	f(testSetupMCM_Leader_Figure7LeaderLine)
}

// Extra: raft violation - only leader can get AppendEntriesReply
func TestCM_RpcAER_FollowerOrCandidate_ReturnsErrorForSameTermReply(t *testing.T) {
	f := func(setup func(t *testing.T) (mcm *managedConsensusModule, mrs *mockRpcSender)) {
		mcm, _ := setup(t)
		serverTerm := mcm.pcm.persistentState.GetCurrentTerm()
		sentRpc := makeAEWithTerm(serverTerm)

		err := mcm.pcm.rpcReply_RpcAppendEntriesReply(
			"s2",
			sentRpc,
			&RpcAppendEntriesReply{serverTerm, true},
		)
		expectedErr := fmt.Sprintf(
			"FATAL: non-leader got AppendEntriesReply from: s2 with term: %v",
			serverTerm,
		)
		if err.Error() != expectedErr {
			t.Fatal(err)
		}
	}

	f(testSetupMCM_Follower_Figure7LeaderLine)
	f(testSetupMCM_Candidate_Figure7LeaderLine)
}

// #RFS-A2: If RPC request or response contains term T > currentTerm:
// set currentTerm = T, convert to follower (#5.1)
// #5.1-p3s4: ...; if one server's current term is smaller than the
// other's, then it updates its current term to the larger value.
// #5.1-p3s5: If a candidate or leader discovers that its term is out of
// date, it immediately reverts to follower state.
func TestCM_RpcAER_Leader_NewerTerm(t *testing.T) {
	mcm, mrs := testSetupMCM_Leader_Figure7LeaderLine(t)
	serverTerm := mcm.pcm.persistentState.GetCurrentTerm()
	sentRpc := makeAEWithTerm(serverTerm)

	// sanity check
	expectedNextIndex := map[ServerId]LogIndex{"s2": 11, "s3": 11, "s4": 11, "s5": 11}
	if !reflect.DeepEqual(mcm.pcm.leaderVolatileState.nextIndex, expectedNextIndex) {
		t.Fatal()
	}
	expectedMatchIndex := map[ServerId]LogIndex{"s2": 0, "s3": 0, "s4": 0, "s5": 0}
	if !reflect.DeepEqual(mcm.pcm.leaderVolatileState.matchIndex, expectedMatchIndex) {
		t.Fatal()
	}

	err := mcm.pcm.rpcReply_RpcAppendEntriesReply(
		"s2",
		sentRpc,
		&RpcAppendEntriesReply{serverTerm + 1, false},
	)
	if err != nil {
		t.Fatal(err)
	}
	if mcm.pcm.getServerState() != FOLLOWER {
		t.Fatal()
	}
	if mcm.pcm.persistentState.GetCurrentTerm() != serverTerm+1 {
		t.Fatal()
	}
	if mcm.pcm.persistentState.GetVotedFor() != "" {
		t.Fatal()
	}

	// no other changes
	if !reflect.DeepEqual(mcm.pcm.leaderVolatileState.nextIndex, expectedNextIndex) {
		t.Fatal()
	}
	if !reflect.DeepEqual(mcm.pcm.leaderVolatileState.matchIndex, expectedMatchIndex) {
		t.Fatal()
	}
	expectedRpcs := []mockSentRpc{}
	mrs.checkSentRpcs(t, expectedRpcs)
}

// #RFS-L3.2: If AppendEntries fails because of log inconsistency:
// decrement nextIndex and retry (#5.3)
// #5.3-p8s6: After a rejection, the leader decrements nextIndex and
// retries the AppendEntries RPC.
// Note: test based on Figure 7; server is leader line; peer is case (a)
func TestCM_RpcAER_Leader_ResultIsFail(t *testing.T) {
	mcm, mrs := testSetupMCM_Leader_Figure7LeaderLine(t)
	serverTerm := mcm.pcm.persistentState.GetCurrentTerm()
	err := mcm.pcm.setCommitIndex(3)
	if err != nil {
		t.Fatal(err)
	}

	// sanity check
	expectedNextIndex := map[ServerId]LogIndex{"s2": 11, "s3": 11, "s4": 11, "s5": 11}
	if !reflect.DeepEqual(mcm.pcm.leaderVolatileState.nextIndex, expectedNextIndex) {
		t.Fatal()
	}
	expectedMatchIndex := map[ServerId]LogIndex{"s2": 0, "s3": 0, "s4": 0, "s5": 0}
	if !reflect.DeepEqual(mcm.pcm.leaderVolatileState.matchIndex, expectedMatchIndex) {
		t.Fatal()
	}

	sentRpc := &RpcAppendEntries{
		serverTerm,
		10,
		6,
		[]LogEntry{},
		mcm.pcm.getCommitIndex(),
	}

	err = mcm.pcm.rpcReply_RpcAppendEntriesReply(
		"s3",
		sentRpc,
		&RpcAppendEntriesReply{serverTerm, false},
	)
	if err != nil {
		t.Fatal(err)
	}
	if mcm.pcm.getServerState() != LEADER {
		t.Fatal()
	}
	if mcm.pcm.persistentState.GetCurrentTerm() != serverTerm {
		t.Fatal()
	}
	expectedNextIndex = map[ServerId]LogIndex{"s2": 11, "s3": 10, "s4": 11, "s5": 11}
	if !reflect.DeepEqual(mcm.pcm.leaderVolatileState.nextIndex, expectedNextIndex) {
		t.Fatal()
	}
	expectedMatchIndex = map[ServerId]LogIndex{"s2": 0, "s3": 0, "s4": 0, "s5": 0}
	if !reflect.DeepEqual(mcm.pcm.leaderVolatileState.matchIndex, expectedMatchIndex) {
		t.Fatal()
	}
	//
	expectedRpc := &RpcAppendEntries{
		serverTerm,
		9,
		6,
		[]LogEntry{
			{6, Command("c10")},
		},
		3,
	}
	expectedRpcs := []mockSentRpc{
		{"s3", expectedRpc},
	}
	mrs.checkSentRpcs(t, expectedRpcs)
}

// #RFS-L3.1: If successful: update nextIndex and matchIndex for
// follower (#5.3)
// Note: test based on Figure 7; server is leader line; peer is the same
func TestCM_RpcAER_Leader_ResultIsSuccess_UpToDatePeer(t *testing.T) {
	mcm, mrs := testSetupMCM_Leader_Figure7LeaderLine(t)
	serverTerm := mcm.pcm.persistentState.GetCurrentTerm()
	err := mcm.pcm.setCommitIndex(3)
	if err != nil {
		t.Fatal(err)
	}

	// sanity check
	expectedNextIndex := map[ServerId]LogIndex{"s2": 11, "s3": 11, "s4": 11, "s5": 11}
	if !reflect.DeepEqual(mcm.pcm.leaderVolatileState.nextIndex, expectedNextIndex) {
		t.Fatal()
	}
	expectedMatchIndex := map[ServerId]LogIndex{"s2": 0, "s3": 0, "s4": 0, "s5": 0}
	if !reflect.DeepEqual(mcm.pcm.leaderVolatileState.matchIndex, expectedMatchIndex) {
		t.Fatal()
	}

	sentRpc := &RpcAppendEntries{
		serverTerm,
		10,
		6,
		[]LogEntry{},
		mcm.pcm.getCommitIndex(),
	}

	err = mcm.pcm.rpcReply_RpcAppendEntriesReply(
		"s3",
		sentRpc,
		&RpcAppendEntriesReply{serverTerm, true},
	)
	if err != nil {
		t.Fatal(err)
	}
	if mcm.pcm.getServerState() != LEADER {
		t.Fatal()
	}
	if mcm.pcm.persistentState.GetCurrentTerm() != serverTerm {
		t.Fatal()
	}
	expectedNextIndex = map[ServerId]LogIndex{"s2": 11, "s3": 11, "s4": 11, "s5": 11}
	if !reflect.DeepEqual(mcm.pcm.leaderVolatileState.nextIndex, expectedNextIndex) {
		t.Fatal()
	}
	expectedMatchIndex = map[ServerId]LogIndex{"s2": 0, "s3": 10, "s4": 0, "s5": 0}
	if !reflect.DeepEqual(mcm.pcm.leaderVolatileState.matchIndex, expectedMatchIndex) {
		t.Fatal()
	}
	// no new follow-on AppendEntries expected
	expectedRpcs := []mockSentRpc{}
	mrs.checkSentRpcs(t, expectedRpcs)
}

// #RFS-L3.1: If successful: update nextIndex and matchIndex for
// follower (#5.3)
// #RFS-L4: If there exists an N such that N > commitIndex, a majority
// of matchIndex[i] >= N, and log[N].term == currentTerm:
// set commitIndex = N (#5.3, #5.4)
// Note: test based on Figure 7; server is leader line; peer is case (a)
func TestCM_RpcAER_Leader_ResultIsSuccess_PeerJustCaughtUp(t *testing.T) {
	mcm, mrs := testSetupMCM_Leader_Figure7LeaderLine(t)
	serverTerm := mcm.pcm.persistentState.GetCurrentTerm()
	err := mcm.pcm.setCommitIndex(3)
	if err != nil {
		t.Fatal(err)
	}

	// hack & sanity check
	mcm.pcm.leaderVolatileState.nextIndex["s2"] = 10
	expectedNextIndex := map[ServerId]LogIndex{"s2": 10, "s3": 11, "s4": 11, "s5": 11}
	if !reflect.DeepEqual(mcm.pcm.leaderVolatileState.nextIndex, expectedNextIndex) {
		t.Fatal()
	}
	expectedMatchIndex := map[ServerId]LogIndex{"s2": 0, "s3": 0, "s4": 0, "s5": 0}
	if !reflect.DeepEqual(mcm.pcm.leaderVolatileState.matchIndex, expectedMatchIndex) {
		t.Fatal()
	}

	sentRpc := &RpcAppendEntries{
		serverTerm,
		9,
		6,
		[]LogEntry{
			{6, Command("c10")},
		},
		mcm.pcm.getCommitIndex(),
	}

	err = mcm.pcm.rpcReply_RpcAppendEntriesReply(
		"s2",
		sentRpc,
		&RpcAppendEntriesReply{serverTerm, true},
	)
	if err != nil {
		t.Fatal(err)
	}
	if mcm.pcm.getServerState() != LEADER {
		t.Fatal()
	}
	if mcm.pcm.persistentState.GetCurrentTerm() != serverTerm {
		t.Fatal()
	}
	expectedNextIndex = map[ServerId]LogIndex{"s2": 11, "s3": 11, "s4": 11, "s5": 11}
	if !reflect.DeepEqual(mcm.pcm.leaderVolatileState.nextIndex, expectedNextIndex) {
		t.Fatal(mcm.pcm.leaderVolatileState.nextIndex)
	}
	expectedMatchIndex = map[ServerId]LogIndex{"s2": 10, "s3": 0, "s4": 0, "s5": 0}
	if !reflect.DeepEqual(mcm.pcm.leaderVolatileState.matchIndex, expectedMatchIndex) {
		t.Fatal()
	}

	// let's make some new log entries
	li, err := mcm.pcm.appendCommand(Command("c11"))
	if li != 11 || err != nil {
		t.Fatal()
	}
	li, err = mcm.pcm.appendCommand(Command("c12"))
	if li != 12 || err != nil {
		t.Fatal()
	}
	// we currently do not expect appendCommand() to send AppendEntries
	expectedRpcs := []mockSentRpc{}
	mrs.checkSentRpcs(t, expectedRpcs)

	// rpcs should go out on tick
	expectedRpc := &RpcAppendEntries{serverTerm, 10, 6, []LogEntry{
		{8, Command("c11")},
		{8, Command("c12")},
	}, 3}
	expectedRpcs = []mockSentRpc{
		{"s2", expectedRpc},
		{"s3", expectedRpc},
		{"s4", expectedRpc},
		{"s5", expectedRpc},
	}
	err = mcm.tick()
	if err != nil {
		t.Fatal(err)
	}
	mrs.checkSentRpcs(t, expectedRpcs)

	// one reply - cannot advance commitIndex
	err = mcm.pcm.rpcReply_RpcAppendEntriesReply(
		"s2",
		expectedRpc,
		&RpcAppendEntriesReply{serverTerm, true},
	)
	if err != nil {
		t.Fatal(err)
	}
	if ci := mcm.pcm.getCommitIndex(); ci != 3 {
		t.Fatal(ci)
	}

	// another reply - can advance commitIndex with majority
	err = mcm.pcm.rpcReply_RpcAppendEntriesReply(
		"s4",
		expectedRpc,
		&RpcAppendEntriesReply{serverTerm, true},
	)
	if err != nil {
		t.Fatal(err)
	}
	if ci := mcm.pcm.getCommitIndex(); ci != 11 {
		t.Fatal(ci)
	}

	// other checks
	if mcm.pcm.getServerState() != LEADER {
		t.Fatal()
	}
	expectedNextIndex = map[ServerId]LogIndex{"s2": 13, "s3": 11, "s4": 13, "s5": 11}
	if !reflect.DeepEqual(mcm.pcm.leaderVolatileState.nextIndex, expectedNextIndex) {
		t.Fatal(mcm.pcm.leaderVolatileState.nextIndex)
	}
	expectedMatchIndex = map[ServerId]LogIndex{"s2": 12, "s3": 0, "s4": 12, "s5": 0}
	if !reflect.DeepEqual(mcm.pcm.leaderVolatileState.matchIndex, expectedMatchIndex) {
		t.Fatal(mcm.pcm.leaderVolatileState.matchIndex)
	}
}
