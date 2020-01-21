package logindex

import (
	"errors"
	"sync"

	. "github.com/divtxt/raft"
)

type IndexChangeVerifier func(old, new LogIndex) error

// WatchedIndex is a LogIndex that implements WatchableIndex.
type WatchedIndex struct {
	lock      sync.Locker
	value     LogIndex
	verifier  IndexChangeVerifier
	listeners []IndexChangeListener
}

// NewWatchedIndex creates a new WatchedIndex that uses the given Locker
// to ensure safe concurrent access.
// The initial LogIndex value will be 0.
func NewWatchedIndex(lock sync.Locker) *WatchedIndex {
	return &WatchedIndex{
		lock:      lock,
		value:     0,
		verifier:  nil,
		listeners: nil,
	}
}

// Get the current value.
// This will use the underlying Locker to ensure safe concurrent access.
func (p *WatchedIndex) Get() LogIndex {
	p.lock.Lock()
	v := p.value
	p.lock.Unlock()
	return v
}

// UnsafeGet gets the current value WITHOUT locking the underlying Locker.
// The caller MUST have locked the underlying Locker to ensure safe concurrent operation.
func (p *WatchedIndex) UnsafeGet() LogIndex {
	return p.value
}

// Set the verifier for changes.
// Returns an error if a verifier has already been set.
func (p *WatchedIndex) SetVerifier(verifier IndexChangeVerifier) error {
	p.lock.Lock()
	if p.verifier != nil {
		return errors.New("verifier already set")
	}
	p.verifier = verifier
	p.lock.Unlock()
	return nil
}

// Add the given callback as a listener for changes.
// This will use the underlying Locker to ensure safe concurrent access.
//
// Whenever the underlying value changes, all listeners will be called in order.
// Any listener can indicate an error in the change and this will be treated as fatal.
func (p *WatchedIndex) AddListener(didChangeListener IndexChangeListener) {
	p.lock.Lock()
	p.listeners = append(p.listeners, didChangeListener)
	p.lock.Unlock()
}

// UnsafeSet sets the LogIndex to the given value WITHOUT locking the underlying Locker.
//
// The caller MUST have locked the underlying Locker to ensure safe concurrent operation.
//
// If a verifier is set, the value change is then checked with the verifier.
// If the verifier returns an error this error is returned.
// This is expected to be fatal for the caller.
//
// If the value is accepted by the verifier, all registered listeners are called in order.
//
// Since the underlying lock should be held during this call, the verifier and listeners
// are guaranteed that another change will not occur until they have returned to this method.
// However, this also means that the verifier or listeners will block all other callers
// to this WatchedIndex until they return.
func (p *WatchedIndex) UnsafeSet(new LogIndex) error {
	old := p.value
	p.value = new

	if p.verifier != nil {
		err := p.verifier(old, new)
		if err != nil {
			return err
		}
	}

	for _, f := range p.listeners {
		f(new)
	}
	return nil
}
