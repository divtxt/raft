package testhelpers

import (
	"fmt"

	. "github.com/divtxt/raft"
)

// MockCommitIndexChangeListener is a mock implementation of CommitIndexChangeListener.
type MockCommitIndexChangeListener struct {
	commitIndex LogIndex
}

// NewMockCommitIndexChangeListener creates a new MockCommitIndexChangeListener.
func NewMockCommitIndexChangeListener(
// FIXME: take commitIndex ?
) *MockCommitIndexChangeListener {
	return &MockCommitIndexChangeListener{0}
}

func (mcl *MockCommitIndexChangeListener) CommitIndexChanged(commitIndex LogIndex) {
	if commitIndex < mcl.commitIndex {
		panic(fmt.Sprintf(
			"MockCommitIndexChangeListener: CommitIndexChanged(%d) is < current commitIndex=%d",
			commitIndex,
			mcl.commitIndex,
		))
	}
	mcl.commitIndex = commitIndex
}

func (mcl *MockCommitIndexChangeListener) GetCommitIndex() LogIndex {
	return mcl.commitIndex
}
