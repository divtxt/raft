package logindex_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/logindex"
)

func TestWatchedIndex(t *testing.T) {
	wi := logindex.NewWatchedIndex()

	ss := &someStrings{}

	// Initial value should be 0
	if wi.Get() != 0 {
		t.Fatal()
	}

	// Set - uses Locker
	err := wi.Set(3)
	if err != nil {
		t.Fatal(err)
	}
	ss.checkCalls(t, []string{})

	// Get - does not use Locker
	if wi.Get() != 3 {
		t.Fatal()
	}

	// Add a listener & check that it is called on Set
	var icl1 IndexChangeListener = func(n LogIndex) {
		ss.append(fmt.Sprintf("icl1:%v", n))
	}
	wi.AddListener(icl1)
	err = wi.Set(4)
	if err != nil {
		t.Fatal(err)
	}
	ss.checkCalls(t, []string{"icl1:4"})

	// Second listener
	var icl2 IndexChangeListener = func(n LogIndex) {
		ss.append(fmt.Sprintf("icl2:%v", n))
	}
	wi.AddListener(icl2)
	err = wi.Set(8)
	if err != nil {
		t.Fatal(err)
	}
	ss.checkCalls(t, []string{"icl1:8", "icl2:8"})
}

func TestWatchedIndex_With_Verifier(t *testing.T) {
	ss := &someStrings{}

	var fErr = errors.New("fErr")
	var icv logindex.IndexChangeVerifier = func(o, n LogIndex) error {
		ss.append(fmt.Sprintf("icv:%v->%v", o, n))
		if n == 10 {
			return fErr
		}
		return nil
	}

	wi := logindex.NewWatchedIndexWithVerifier(icv)

	// Initial value should be 0
	if wi.Get() != 0 {
		t.Fatal()
	}

	// Add a listener
	var icl1 IndexChangeListener = func(n LogIndex) {
		ss.append(fmt.Sprintf("icl1:%v", n))
	}
	wi.AddListener(icl1)

	// Set should call verifier and listener
	err := wi.Set(5)
	if err != nil {
		t.Fatal(err)
	}
	ss.checkCalls(t, []string{"icv:0->5", "icl1:5"})

	// Set should return error without calling listeners if the verifier errors
	err = wi.Set(10)
	if err != fErr {
		t.Fatal(err)
	}
	ss.checkCalls(t, []string{"icv:5->10"})

	// New value should NOT have been set when the verifier returns an error
	if wi.Get() != 5 {
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
	if ss.l == nil {
		if len(expected) != 0 {
			t.Fatal(ss.l, expected)
		}
	} else if !reflect.DeepEqual(ss.l, expected) {
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
