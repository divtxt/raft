package logindex

import (
	"sync"
	"sync/atomic"

	. "github.com/divtxt/raft"
)

type IndexChangeVerifier func(old, new LogIndex) error

// WatchedIndex is a LogIndex that is checked and notifies listeners when set.
type WatchedIndex struct {
	lock      sync.Mutex
	value     LogIndex
	verifier  IndexChangeVerifier
	listeners []IndexChangeListener
}

// NewWatchedIndex creates a new WatchedIndex without a verifier.
// The initial LogIndex value will be 0.
func NewWatchedIndex() *WatchedIndex {
	return NewWatchedIndexWithVerifier(nil)
}

// NewWatchedIndex creates a new WatchedIndex with the given verifier.
// The initial LogIndex value will be 0.
func NewWatchedIndexWithVerifier(verifier IndexChangeVerifier) *WatchedIndex {
	return &WatchedIndex{
		value:     0,
		verifier:  verifier,
		listeners: nil,
	}
}

// Get the current value.
// This uses atomic.LoadUint64 to ensure safe concurrent access with Set().
// It does NOT lock the Mutex.
func (p *WatchedIndex) Get() LogIndex {
	return LogIndex(atomic.LoadUint64((*uint64)(&p.value)))
}

// Add the given callback as a listener for changes.
//
// This will use the underlying Mutex to ensure safe concurrent modification of
// the value or the list of listeners which means that it is NOT safe to call
// from the verifier or a listener.
//
// Whenever the underlying value changes, the listener will be called.
func (p *WatchedIndex) AddListener(didChangeListener IndexChangeListener) {
	p.lock.Lock()
	p.listeners = append(p.listeners, didChangeListener)
	p.lock.Unlock()
}

// Set the LogIndex to the given value.
//
// This will use the underlying Mutex to ensure safe concurrent modification of the
// value list of listeners which means that it is NOT safe to call from a listener.
//
// If a verifier is set, the value change is first checked with the verifier.
// If the verifier returns an error, that error is returned.
// This is expected to be fatal for the caller.
//
// If the value is accepted by the verifier, the value is changed using
// atomic.StoreUint64 to ensure safe concurrent access with Get().
//
// After the value is changed, all listeners are called in registered order.
//
// Since the underlying lock is held during the call, the verifier and listeners
// are guaranteed that another change will not occur until they have returned to this method.
// However, this also means that the verifier or listeners will block calls to Set()
// or AddListener() until they return.
func (p *WatchedIndex) Set(new LogIndex) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	if p.verifier != nil {
		err := p.verifier(p.value, new)
		if err != nil {
			return err
		}
	}

	atomic.StoreUint64((*uint64)(&p.value), uint64(new))

	for _, f := range p.listeners {
		f(new)
	}
	return nil
}
