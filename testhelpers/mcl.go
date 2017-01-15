package testhelpers

import (
	"fmt"

	. "github.com/divtxt/raft"
)

// MockChangeListener is a mock implementation of ChangeListener.
type MockChangeListener struct {
	commitIndex LogIndex
}

// NewMockChangeListener creates a new MockChangeListener
func NewMockChangeListener(
// FIXME: take commitIndex ?
) *MockChangeListener {
	return &MockChangeListener{0}
}

func (mcl *MockChangeListener) CommitIndexChanged(commitIndex LogIndex) {
	if commitIndex < mcl.commitIndex {
		panic(fmt.Sprintf(
			"MockChangeListener: CommitIndexChanged(%d) is < current commitIndex=%d",
			commitIndex,
			mcl.commitIndex,
		))
	}
	mcl.commitIndex = commitIndex
}

func (mcl *MockChangeListener) GetCommitIndex() LogIndex {
	return mcl.commitIndex
}
