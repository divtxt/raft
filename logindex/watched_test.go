package logindex_test

import (
	"errors"
	"fmt"
	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/logindex"
	"reflect"
	"testing"
)

func TestWatchedIndex(t *testing.T) {
	ss := &someStrings{}
	l := &mockLocker{ss}

	var fErr = errors.New("fErr")
	var icl1 IndexChangeListener = func(o, n LogIndex) error {
		ss.append(fmt.Sprintf("icl1:%v->%v", o, n))
		if n == 10 {
			return fErr
		}
		return nil
	}
	var icl2 IndexChangeListener = func(o, n LogIndex) error {
		ss.append(fmt.Sprintf("icl2:%v->%v", o, n))
		return nil
	}

	wi := logindex.NewWatchedIndex(l)

	ss.checkCalls(t, nil)

	// Initial value should be 0
	if wi.Get() != 0 {
		t.Fatal()
	}
	ss.checkCalls(t, []string{"Lock", "Unlock"})

	// UnsafeSet
	err := wi.UnsafeSet(3)
	if err != nil {
		t.Fatal(err)
	}
	ss.checkCalls(t, nil)
	if wi.Get() != 3 {
		t.Fatal()
	}
	ss.checkCalls(t, []string{"Lock", "Unlock"})

	// UnsafeGet
	if wi.UnsafeGet() != 3 {
		t.Fatal()
	}
	ss.checkCalls(t, nil)

	// Add a listener
	wi.AddListener(icl1)
	ss.checkCalls(t, []string{"Lock", "Unlock"})
	err = wi.UnsafeSet(4)
	if err != nil {
		t.Fatal(err)
	}
	ss.checkCalls(t, []string{"icl1:3->4"})

	// Second listener
	wi.AddListener(icl2)
	ss.checkCalls(t, []string{"Lock", "Unlock"})
	err = wi.UnsafeSet(8)
	if err != nil {
		t.Fatal(err)
	}
	ss.checkCalls(t, []string{"icl1:4->8", "icl2:4->8"})

	// UnsafeSet should return when first listener errors
	err = wi.UnsafeSet(10)
	if err != fErr {
		t.Fatal(err)
	}
	ss.checkCalls(t, []string{"icl1:8->10"})

	// New value should have been set regardless of the error
	if wi.Get() != 10 {
		t.Fatal()
	}
}

type someStrings struct {
	l []string
}

func (ss *someStrings) append(s string) {
	ss.l = append(ss.l, s)
}

func (ss *someStrings) checkCalls(t *testing.T, expected []string) {
	if !reflect.DeepEqual(ss.l, expected) {
		t.Fatal(ss.l, expected)
	}
	ss.l = nil
}

type mockLocker struct {
	ss *someStrings
}

func (l *mockLocker) Lock() {
	l.ss.append("Lock")
}

func (l *mockLocker) Unlock() {
	l.ss.append("Unlock")
}
