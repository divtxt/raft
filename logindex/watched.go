package logindex

import (
	"sync"

	. "github.com/divtxt/raft"
)

type Getter func() LogIndex

type ChangeListener func(LogIndex) error

// WatchedIndex is a LogIndex whose changes can be subscribed to and validated.
type WatchedIndex struct {
	lock     sync.Locker
	value    LogIndex
	listener ChangeListener
}

// NewWatchedIndex creates a new WatchedIndex that uses the given lock and given listener.
// The value of the LogIndex will be 0.
func NewWatchedIndex(lock sync.Locker, listener ChangeListener) *WatchedIndex {
	return &WatchedIndex{
		lock,
		0,
		listener,
	}
}

//// RegisterListener registers the given listener for changes.
//func (p *WatchedIndex) RegisterListener(f ChangeListener) {
//	p.lock.Lock()
//	p.listeners = append(p.listeners, f)
//	p.lock.Unlock()
//}

// Get the current value.
// The value is read under the lock specified in NewWatchedIndex.
func (p *WatchedIndex) Get() LogIndex {
	p.lock.Lock()
	v := p.value
	p.lock.Unlock()
	return v
}

// Set the WatchedIndex to the given value.
// The value is written under the lock specified in NewWatchedIndex.
//
// After the value is changed, the listener is called with the new value
// and Set returns any error it returns.
// Note that the lock is NOT held when the listener is called.
func (p *WatchedIndex) Set(newValue LogIndex) (err error) {
	p.lock.Lock()
	p.value = newValue
	p.lock.Unlock()
	if p.listener != nil {
		err = p.listener(newValue)
		if err != nil {
			return err
		}
	}
	return nil
}
