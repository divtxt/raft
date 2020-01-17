package consensus

import (
	"fmt"
	"reflect"

	. "github.com/divtxt/raft"
)

type indexWatcher struct {
	changes []string
}

func newIndexWatcher(wi WatchableIndex) *indexWatcher {
	iw := &indexWatcher{}
	wi.AddListener(iw.indexChanged)
	return iw
}

func (iw *indexWatcher) indexChanged(old, new LogIndex) error {
	iw.changes = append(iw.changes, fmt.Sprintf("%v->%v", old, new))
	return nil
}

func (iw *indexWatcher) CheckCalls(expected ...string) {
	if len(iw.changes) == 0 && len(expected) == 0 {
		return
	}
	if !reflect.DeepEqual(iw.changes, expected) {
		panic(fmt.Sprintf("%v != %v", iw.changes, expected))
	}
	iw.changes = nil
}
