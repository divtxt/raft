package consensus

import (
	"fmt"
	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/lasm"
	"github.com/divtxt/raft/testhelpers"
	"reflect"
	"testing"
)

// Extra: ignore replies for previous term rpc
func TestCM_RpcAER_All_IgnorePreviousTermRpc(t *testing.T) {
	f := func(setup func(t *testing.T) (mcm *managedConsensusModule, mrs *testhelpers.MockRpcSender)) {
		mcm, mrs := setup(t)
		serverTerm := mcm.pcm.RaftPersistentState.GetCurrentTerm()
		sentRpc := makeAEWithTerm(serverTerm - 1)
		beforeState := mcm.pcm.GetServerState()

		err := mcm.pcm.RpcReply_RpcAppendEntriesReply(
			"s2",
			sentRpc,
			&RpcAppendEntriesReply{serverTerm, true},
		)
		if err != nil {
			t.Fatal(err)
		}
		if mcm.pcm.GetServerState() != beforeState {
			t.Fatal()
		}
		if mcm.pcm.RaftPersistentState.GetCurrentTerm() != serverTerm {
			t.Fatal()
		}
		if beforeState == LEADER {
			expectedNextIndex := map[ServerId]LogIndex{"s2": 11, "s3": 11, "s4": 11, "s5": 11}
			if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.NextIndex, expectedNextIndex) {
				t.Fatal()
			}
			expectedMatchIndex := map[ServerId]LogIndex{"s2": 0, "s3": 0, "s4": 0, "s5": 0}
			if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.MatchIndex, expectedMatchIndex) {
				t.Fatal()
			}
			mrs.CheckSentRpcs(t, []testhelpers.MockSentRpc{})
		}
	}

	f(testSetupMCM_Follower_Figure7LeaderLine)
	f(testSetupMCM_Candidate_Figure7LeaderLine)
	f(testSetupMCM_Leader_Figure7LeaderLine)
}

// Extra: raft violation - only leader can get AppendEntriesReply
func TestCM_RpcAER_FollowerOrCandidate_ReturnsErrorForSameTermReply(t *testing.T) {
	f := func(setup func(t *testing.T) (mcm *managedConsensusModule, mrs *testhelpers.MockRpcSender)) {
		mcm, _ := setup(t)
		serverTerm := mcm.pcm.RaftPersistentState.GetCurrentTerm()
		sentRpc := makeAEWithTerm(serverTerm)

		err := mcm.pcm.RpcReply_RpcAppendEntriesReply(
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
	serverTerm := mcm.pcm.RaftPersistentState.GetCurrentTerm()
	sentRpc := makeAEWithTerm(serverTerm)

	// sanity check
	expectedNextIndex := map[ServerId]LogIndex{"s2": 11, "s3": 11, "s4": 11, "s5": 11}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.NextIndex, expectedNextIndex) {
		t.Fatal()
	}
	expectedMatchIndex := map[ServerId]LogIndex{"s2": 0, "s3": 0, "s4": 0, "s5": 0}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.MatchIndex, expectedMatchIndex) {
		t.Fatal()
	}

	err := mcm.pcm.RpcReply_RpcAppendEntriesReply(
		"s2",
		sentRpc,
		&RpcAppendEntriesReply{serverTerm + 1, false},
	)
	if err != nil {
		t.Fatal(err)
	}
	if mcm.pcm.GetServerState() != FOLLOWER {
		t.Fatal()
	}
	if mcm.pcm.RaftPersistentState.GetCurrentTerm() != serverTerm+1 {
		t.Fatal()
	}
	if mcm.pcm.RaftPersistentState.GetVotedFor() != "" {
		t.Fatal()
	}

	// no other changes
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.NextIndex, expectedNextIndex) {
		t.Fatal()
	}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.MatchIndex, expectedMatchIndex) {
		t.Fatal()
	}
	expectedRpcs := []testhelpers.MockSentRpc{}
	mrs.CheckSentRpcs(t, expectedRpcs)
}

// #RFS-L3.2: If AppendEntries fails because of log inconsistency:
// decrement nextIndex and retry (#5.3)
// #5.3-p8s6: After a rejection, the leader decrements nextIndex and
// retries the AppendEntries RPC.
// Note: test based on Figure 7; server is leader line; peer is case (a)
func TestCM_RpcAER_Leader_ResultIsFail(t *testing.T) {
	mcm, mrs := testSetupMCM_Leader_Figure7LeaderLine(t)
	serverTerm := mcm.pcm.RaftPersistentState.GetCurrentTerm()
	err := mcm.pcm.setCommitIndex(3)
	if err != nil {
		t.Fatal(err)
	}

	// sanity check
	expectedNextIndex := map[ServerId]LogIndex{"s2": 11, "s3": 11, "s4": 11, "s5": 11}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.NextIndex, expectedNextIndex) {
		t.Fatal()
	}
	expectedMatchIndex := map[ServerId]LogIndex{"s2": 0, "s3": 0, "s4": 0, "s5": 0}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.MatchIndex, expectedMatchIndex) {
		t.Fatal()
	}

	sentRpc := &RpcAppendEntries{
		serverTerm,
		10,
		6,
		[]LogEntry{},
		mcm.pcm.GetCommitIndex(),
	}

	err = mcm.pcm.RpcReply_RpcAppendEntriesReply(
		"s3",
		sentRpc,
		&RpcAppendEntriesReply{serverTerm, false},
	)
	if err != nil {
		t.Fatal(err)
	}
	if mcm.pcm.GetServerState() != LEADER {
		t.Fatal()
	}
	if mcm.pcm.RaftPersistentState.GetCurrentTerm() != serverTerm {
		t.Fatal()
	}
	expectedNextIndex = map[ServerId]LogIndex{"s2": 11, "s3": 10, "s4": 11, "s5": 11}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.NextIndex, expectedNextIndex) {
		t.Fatal()
	}
	expectedMatchIndex = map[ServerId]LogIndex{"s2": 0, "s3": 0, "s4": 0, "s5": 0}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.MatchIndex, expectedMatchIndex) {
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
	expectedRpcs := []testhelpers.MockSentRpc{
		{"s3", expectedRpc},
	}
	mrs.CheckSentRpcs(t, expectedRpcs)
}

// #RFS-L3.1: If successful: update nextIndex and matchIndex for
// follower (#5.3)
// Note: test based on Figure 7; server is leader line; peer is the same
func TestCM_RpcAER_Leader_ResultIsSuccess_UpToDatePeer(t *testing.T) {
	mcm, mrs := testSetupMCM_Leader_Figure7LeaderLine(t)
	serverTerm := mcm.pcm.RaftPersistentState.GetCurrentTerm()
	err := mcm.pcm.setCommitIndex(3)
	if err != nil {
		t.Fatal(err)
	}

	// sanity check
	expectedNextIndex := map[ServerId]LogIndex{"s2": 11, "s3": 11, "s4": 11, "s5": 11}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.NextIndex, expectedNextIndex) {
		t.Fatal()
	}
	expectedMatchIndex := map[ServerId]LogIndex{"s2": 0, "s3": 0, "s4": 0, "s5": 0}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.MatchIndex, expectedMatchIndex) {
		t.Fatal()
	}

	sentRpc := &RpcAppendEntries{
		serverTerm,
		10,
		6,
		[]LogEntry{},
		mcm.pcm.GetCommitIndex(),
	}

	err = mcm.pcm.RpcReply_RpcAppendEntriesReply(
		"s3",
		sentRpc,
		&RpcAppendEntriesReply{serverTerm, true},
	)
	if err != nil {
		t.Fatal(err)
	}
	if mcm.pcm.GetServerState() != LEADER {
		t.Fatal()
	}
	if mcm.pcm.RaftPersistentState.GetCurrentTerm() != serverTerm {
		t.Fatal()
	}
	expectedNextIndex = map[ServerId]LogIndex{"s2": 11, "s3": 11, "s4": 11, "s5": 11}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.NextIndex, expectedNextIndex) {
		t.Fatal()
	}
	expectedMatchIndex = map[ServerId]LogIndex{"s2": 0, "s3": 10, "s4": 0, "s5": 0}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.MatchIndex, expectedMatchIndex) {
		t.Fatal()
	}
	// no new follow-on AppendEntries expected
	expectedRpcs := []testhelpers.MockSentRpc{}
	mrs.CheckSentRpcs(t, expectedRpcs)
}

// #RFS-L3.1: If successful: update nextIndex and matchIndex for
// follower (#5.3)
// #RFS-L4: If there exists an N such that N > commitIndex, a majority
// of matchIndex[i] >= N, and log[N].term == currentTerm:
// set commitIndex = N (#5.3, #5.4)
// Note: test based on Figure 7; server is leader line; peer is case (a)
func TestCM_RpcAER_Leader_ResultIsSuccess_PeerJustCaughtUp(t *testing.T) {
	mcm, mrs := testSetupMCM_Leader_Figure7LeaderLine(t)
	serverTerm := mcm.pcm.RaftPersistentState.GetCurrentTerm()
	err := mcm.pcm.setCommitIndex(3)
	if err != nil {
		t.Fatal(err)
	}

	// hack & sanity check
	mcm.pcm.LeaderVolatileState.NextIndex["s2"] = 10
	expectedNextIndex := map[ServerId]LogIndex{"s2": 10, "s3": 11, "s4": 11, "s5": 11}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.NextIndex, expectedNextIndex) {
		t.Fatal()
	}
	expectedMatchIndex := map[ServerId]LogIndex{"s2": 0, "s3": 0, "s4": 0, "s5": 0}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.MatchIndex, expectedMatchIndex) {
		t.Fatal()
	}

	sentRpc := &RpcAppendEntries{
		serverTerm,
		9,
		6,
		[]LogEntry{
			{6, Command("c10")},
		},
		mcm.pcm.GetCommitIndex(),
	}

	err = mcm.pcm.RpcReply_RpcAppendEntriesReply(
		"s2",
		sentRpc,
		&RpcAppendEntriesReply{serverTerm, true},
	)
	if err != nil {
		t.Fatal(err)
	}
	if mcm.pcm.GetServerState() != LEADER {
		t.Fatal()
	}
	if mcm.pcm.RaftPersistentState.GetCurrentTerm() != serverTerm {
		t.Fatal()
	}
	expectedNextIndex = map[ServerId]LogIndex{"s2": 11, "s3": 11, "s4": 11, "s5": 11}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.NextIndex, expectedNextIndex) {
		t.Fatal(mcm.pcm.LeaderVolatileState.NextIndex)
	}
	expectedMatchIndex = map[ServerId]LogIndex{"s2": 10, "s3": 0, "s4": 0, "s5": 0}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.MatchIndex, expectedMatchIndex) {
		t.Fatal()
	}

	// let's make some new log entries
	result, err := mcm.pcm.AppendCommand("c11")
	if result != lasm.DummyInMemoryLasm_AppendEntry_Ok || err != nil {
		t.Fatal()
	}
	result, err = mcm.pcm.AppendCommand("c12")
	if result != lasm.DummyInMemoryLasm_AppendEntry_Ok || err != nil {
		t.Fatal()
	}
	// we currently do not expect appendCommand() to send AppendEntries
	expectedRpcs := []testhelpers.MockSentRpc{}
	mrs.CheckSentRpcs(t, expectedRpcs)

	// rpcs should go out on tick
	expectedRpc := &RpcAppendEntries{serverTerm, 10, 6, []LogEntry{
		{8, Command("c11")},
		{8, Command("c12")},
	}, 3}
	expectedRpcs = []testhelpers.MockSentRpc{
		{"s2", expectedRpc},
		{"s3", expectedRpc},
		{"s4", expectedRpc},
		{"s5", expectedRpc},
	}
	err = mcm.Tick()
	if err != nil {
		t.Fatal(err)
	}
	mrs.CheckSentRpcs(t, expectedRpcs)

	// one reply - cannot advance commitIndex
	err = mcm.pcm.RpcReply_RpcAppendEntriesReply(
		"s2",
		expectedRpc,
		&RpcAppendEntriesReply{serverTerm, true},
	)
	if err != nil {
		t.Fatal(err)
	}
	if ci := mcm.pcm.GetCommitIndex(); ci != 3 {
		t.Fatal(ci)
	}

	// another reply - can advance commitIndex with majority
	// commitIndex will advance to the highest match possible
	err = mcm.pcm.RpcReply_RpcAppendEntriesReply(
		"s4",
		expectedRpc,
		&RpcAppendEntriesReply{serverTerm, true},
	)
	if err != nil {
		t.Fatal(err)
	}
	if ci := mcm.pcm.GetCommitIndex(); ci != 12 {
		t.Fatal(ci)
	}

	// other checks
	if mcm.pcm.GetServerState() != LEADER {
		t.Fatal()
	}
	expectedNextIndex = map[ServerId]LogIndex{"s2": 13, "s3": 11, "s4": 13, "s5": 11}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.NextIndex, expectedNextIndex) {
		t.Fatal(mcm.pcm.LeaderVolatileState.NextIndex)
	}
	expectedMatchIndex = map[ServerId]LogIndex{"s2": 12, "s3": 0, "s4": 12, "s5": 0}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.MatchIndex, expectedMatchIndex) {
		t.Fatal(mcm.pcm.LeaderVolatileState.MatchIndex)
	}
}
