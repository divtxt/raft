package consensus

import (
	"fmt"
	"reflect"
	"testing"

	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/testhelpers"
)

// Extra: ignore replies for previous term rpc
func TestCM_RpcAER_All_IgnorePreviousTermRpc(t *testing.T) {
	f := func(setup func(t *testing.T) (mcm *managedConsensusModule, mrs *testhelpers.MockRpcSender)) {
		mcm, mrs := setup(t)
		serverTerm := mcm.pcm.RaftPersistentState.GetCurrentTerm()
		sentRpc := makeAEWithTerm(serverTerm - 1)
		beforeState := mcm.pcm.GetServerState()

		err := mcm.pcm.RpcReply_RpcAppendEntriesReply(
			102,
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
			expectedNextIndex := map[ServerId]LogIndex{102: 11, 103: 11, 104: 11, 105: 11}
			if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.NextIndexes(), expectedNextIndex) {
				t.Fatal()
			}
			expectedMatchIndex := map[ServerId]LogIndex{102: 0, 103: 0, 104: 0, 105: 0}
			if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.MatchIndexes(), expectedMatchIndex) {
				t.Fatal()
			}
			mrs.CheckSentRpcs(t, map[ServerId]interface{}{})
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
			102,
			sentRpc,
			&RpcAppendEntriesReply{serverTerm, true},
		)
		expectedErr := fmt.Sprintf(
			"FATAL: non-leader got AppendEntriesReply from: 102 with term: %v",
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
	sentRpc := mcm.makeAEWithTerm(102)

	// sanity check
	expectedNextIndex := map[ServerId]LogIndex{102: 11, 103: 11, 104: 11, 105: 11}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.NextIndexes(), expectedNextIndex) {
		t.Fatal()
	}
	expectedMatchIndex := map[ServerId]LogIndex{102: 0, 103: 0, 104: 0, 105: 0}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.MatchIndexes(), expectedMatchIndex) {
		t.Fatal()
	}

	err := mcm.pcm.RpcReply_RpcAppendEntriesReply(
		102,
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
	if mcm.pcm.RaftPersistentState.GetVotedFor() != 0 {
		t.Fatal()
	}

	// no other changes
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.NextIndexes(), expectedNextIndex) {
		t.Fatal()
	}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.MatchIndexes(), expectedMatchIndex) {
		t.Fatal()
	}
	expectedRpcs := map[ServerId]interface{}{}
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
	expectedNextIndex := map[ServerId]LogIndex{102: 11, 103: 11, 104: 11, 105: 11}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.NextIndexes(), expectedNextIndex) {
		t.Fatal()
	}
	expectedMatchIndex := map[ServerId]LogIndex{102: 0, 103: 0, 104: 0, 105: 0}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.MatchIndexes(), expectedMatchIndex) {
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
		103,
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
	expectedNextIndex = map[ServerId]LogIndex{102: 11, 103: 10, 104: 11, 105: 11}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.NextIndexes(), expectedNextIndex) {
		t.Fatal()
	}
	expectedMatchIndex = map[ServerId]LogIndex{102: 0, 103: 0, 104: 0, 105: 0}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.MatchIndexes(), expectedMatchIndex) {
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
	expectedRpcs := map[ServerId]interface{}{
		103: expectedRpc,
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
	expectedNextIndex := map[ServerId]LogIndex{102: 11, 103: 11, 104: 11, 105: 11}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.NextIndexes(), expectedNextIndex) {
		t.Fatal()
	}
	expectedMatchIndex := map[ServerId]LogIndex{102: 0, 103: 0, 104: 0, 105: 0}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.MatchIndexes(), expectedMatchIndex) {
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
		103,
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
	expectedNextIndex = map[ServerId]LogIndex{102: 11, 103: 11, 104: 11, 105: 11}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.NextIndexes(), expectedNextIndex) {
		t.Fatal()
	}
	expectedMatchIndex = map[ServerId]LogIndex{102: 0, 103: 10, 104: 0, 105: 0}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.MatchIndexes(), expectedMatchIndex) {
		t.Fatal()
	}
	// no new follow-on AppendEntries expected
	expectedRpcs := map[ServerId]interface{}{}
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
	fm102, err := mcm.pcm.LeaderVolatileState.GetFollowerManager(102)
	if err != nil {
		t.Fatal(err)
	}
	err = fm102.DecrementNextIndex()
	if err != nil {
		t.Fatal(err)
	}
	expectedNextIndex := map[ServerId]LogIndex{102: 10, 103: 11, 104: 11, 105: 11}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.NextIndexes(), expectedNextIndex) {
		t.Fatal(mcm.pcm.LeaderVolatileState.NextIndexes())
	}
	expectedMatchIndex := map[ServerId]LogIndex{102: 0, 103: 0, 104: 0, 105: 0}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.MatchIndexes(), expectedMatchIndex) {
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
		102,
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
	expectedNextIndex = map[ServerId]LogIndex{102: 11, 103: 11, 104: 11, 105: 11}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.NextIndexes(), expectedNextIndex) {
		t.Fatal(mcm.pcm.LeaderVolatileState.NextIndexes())
	}
	expectedMatchIndex = map[ServerId]LogIndex{102: 10, 103: 0, 104: 0, 105: 0}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.MatchIndexes(), expectedMatchIndex) {
		t.Fatal()
	}

	// let's make some new log entries
	crc11, err := mcm.pcm.AppendCommand(testhelpers.DummyCommand(11))
	if err != nil || crc11 == nil {
		t.Fatal(err)
	}
	crc12, err := mcm.pcm.AppendCommand(testhelpers.DummyCommand(12))
	if err != nil || crc12 == nil {
		t.Fatal(err)
	}
	testhelpers.AssertWillBlock(crc11)
	testhelpers.AssertWillBlock(crc12)

	// we currently do not expect appendCommand() to send AppendEntries
	expectedRpcs := map[ServerId]interface{}{}
	mrs.CheckSentRpcs(t, expectedRpcs)

	// rpcs should go out on tick
	expectedRpc := &RpcAppendEntries{serverTerm, 10, 6, []LogEntry{
		{8, Command("c11")},
		{8, Command("c12")},
	}, 3}
	expectedRpcs = map[ServerId]interface{}{
		102: expectedRpc,
		103: expectedRpc,
		104: expectedRpc,
		105: expectedRpc,
	}
	err = mcm.Tick()
	if err != nil {
		t.Fatal(err)
	}
	mrs.CheckSentRpcs(t, expectedRpcs)

	// one reply - cannot advance commitIndex
	err = mcm.pcm.RpcReply_RpcAppendEntriesReply(
		102,
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
		104,
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
	expectedNextIndex = map[ServerId]LogIndex{102: 13, 103: 11, 104: 13, 105: 11}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.NextIndexes(), expectedNextIndex) {
		t.Fatal(mcm.pcm.LeaderVolatileState.NextIndexes())
	}
	expectedMatchIndex = map[ServerId]LogIndex{102: 12, 103: 0, 104: 12, 105: 0}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.MatchIndexes(), expectedMatchIndex) {
		t.Fatal(mcm.pcm.LeaderVolatileState.MatchIndexes())
	}
}

// Ignore reply for an RpcAppendEntries that does not match the current state.
func TestCM_RpcAER_Leader_IgnoreStateMismatch(t *testing.T) {
	mcm, mrs := testSetupMCM_Leader_Figure7LeaderLine(t)
	serverTerm := mcm.pcm.RaftPersistentState.GetCurrentTerm()
	sentRpc := mcm.makeAEWithTerm(102)

	// reply is handled correctly
	err := mcm.pcm.RpcReply_RpcAppendEntriesReply(
		102,
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
	expectedNextIndex := map[ServerId]LogIndex{102: 10, 103: 11, 104: 11, 105: 11}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.NextIndexes(), expectedNextIndex) {
		t.Fatal()
	}
	expectedMatchIndex := map[ServerId]LogIndex{102: 0, 103: 0, 104: 0, 105: 0}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.MatchIndexes(), expectedMatchIndex) {
		t.Fatal()
	}
	expectedRpc := &RpcAppendEntries{serverTerm, 9, 6, []LogEntry{
		{6, Command("c10")},
	}, 0}
	expectedRpcs := map[ServerId]interface{}{
		102: expectedRpc,
	}
	if err != nil {
		t.Fatal(err)
	}
	mrs.CheckSentRpcs(t, expectedRpcs)
	mrs.ClearSentRpcs()

	// duplicate reply is ignored
	err = mcm.pcm.RpcReply_RpcAppendEntriesReply(
		102,
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
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.NextIndexes(), expectedNextIndex) {
		t.Fatal()
	}
	if !reflect.DeepEqual(mcm.pcm.LeaderVolatileState.MatchIndexes(), expectedMatchIndex) {
		t.Fatal()
	}
	mrs.CheckSentRpcs(t, map[ServerId]interface{}{})
}
