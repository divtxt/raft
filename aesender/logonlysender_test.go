package aesender_test

import (
	"testing"

	. "github.com/divtxt/raft"

	"github.com/divtxt/raft/aesender"
	"github.com/divtxt/raft/internal"
	raft_log "github.com/divtxt/raft/log"
	"github.com/divtxt/raft/testdata"
	"github.com/divtxt/raft/testhelpers"
)

func TestLogOnlyAESender(t *testing.T) {
	iml, err := raft_log.TestUtil_NewInMemoryLog_WithTerms(
		testdata.TestUtil_MakeFigure7LeaderLineTerms(),
		testdata.MaxEntriesPerAppendEntry,
	)
	if err != nil {
		t.Fatal(err)
	}

	err = iml.DiscardEntriesBeforeIndex(5)
	if err != nil {
		t.Fatal(err)
	}

	mrs := testhelpers.NewMockRpcSender()
	aes := aesender.NewLogOnlyAESender(iml, mrs.SendOnlyRpcAppendEntriesAsync)

	var serverTerm TermNo = testdata.CurrentTerm

	// Peer has more entries than this log
	// Currently this is an error!
	params := internal.SendAppendEntriesParams{
		101, 12, false, serverTerm, 4,
	}
	err = aes.SendAppendEntriesToPeerAsync(params)
	if err == nil || err.Error() != "GetTermAtIndex(): li=11 > iole=10" {
		t.Fatal(err)
	}
	mrs.CheckSentRpcs(t, nil)

	// Peer has same number of entries.
	params = internal.SendAppendEntriesParams{
		102, 11, false, serverTerm, 4,
	}
	err = aes.SendAppendEntriesToPeerAsync(params)
	if err != nil {
		t.Fatal(err)
	}
	expectedRpc := &RpcAppendEntries{
		serverTerm,
		10,
		6,
		[]LogEntry{},
		4,
	}
	expectedRpcs := map[ServerId]interface{}{
		102: expectedRpc,
	}
	mrs.CheckSentRpcs(t, expectedRpcs)
	mrs.ClearSentRpcs()
	// Empty send
	params.Empty = true
	err = aes.SendAppendEntriesToPeerAsync(params)
	if err != nil {
		t.Fatal(err)
	}
	mrs.CheckSentRpcs(t, expectedRpcs)
	mrs.ClearSentRpcs()

	// Peer is behind by one entry
	params = internal.SendAppendEntriesParams{
		102, 10, false, serverTerm, 4,
	}
	err = aes.SendAppendEntriesToPeerAsync(params)
	if err != nil {
		t.Fatal(err)
	}
	expectedRpc = &RpcAppendEntries{
		serverTerm,
		9,
		6,
		[]LogEntry{
			{6, Command("c10")},
		},
		4,
	}
	expectedRpcs = map[ServerId]interface{}{
		102: expectedRpc,
	}
	mrs.CheckSentRpcs(t, expectedRpcs)
	mrs.ClearSentRpcs()

	// Peer is behind by multiple entries
	params = internal.SendAppendEntriesParams{
		103, 8, false, serverTerm, 4,
	}
	err = aes.SendAppendEntriesToPeerAsync(params)
	if err != nil {
		t.Fatal(err)
	}
	expectedRpc = &RpcAppendEntries{
		serverTerm,
		7,
		5,
		[]LogEntry{
			{6, Command("c8")},
			{6, Command("c9")},
			{6, Command("c10")},
		},
		4,
	}
	expectedRpcs = map[ServerId]interface{}{
		103: expectedRpc,
	}
	mrs.CheckSentRpcs(t, expectedRpcs)
	mrs.ClearSentRpcs()
	// Empty send
	params.Empty = true
	err = aes.SendAppendEntriesToPeerAsync(params)
	if err != nil {
		t.Fatal(err)
	}
	expectedRpc.Entries = []LogEntry{}
	mrs.CheckSentRpcs(t, expectedRpcs)
	mrs.ClearSentRpcs()

	// Peer is behind log compaction
	params = internal.SendAppendEntriesParams{
		102, 4, false, serverTerm, 4,
	}
	err = aes.SendAppendEntriesToPeerAsync(params)
	if err != ErrIndexCompacted {
		t.Fatal(err)
	}
	mrs.ClearSentRpcs()
}
