package leader

import (
	"testing"
)

func TestFollowerManager(t *testing.T) {
	fm := NewFollowerManager(
		101,
		10,
		9,
		nil,
	)

	if fm.getNextIndex() != 10 || fm.getMatchIndex() != 9 {
		t.Fatal(fm)
	}
}
