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
	ss := &someStrings{}
	l := &mockLocker{ss}

	var fErr = errors.New("fErr")
	var cl logindex.ChangeListener = func(v LogIndex) error {
		ss.append(fmt.Sprintf("%v", v))
		if v == 10 {
			return fErr
		}
		return nil
	}

	p := logindex.NewWatchedIndex(l, cl)

	ss.checkCalls(t, nil)

	if p.Get() != 0 {
		t.Fatal(p)
	}
	ss.checkCalls(t, []string{"Lock", "Unlock"})

	err := p.Set(8)
	if err != nil {
		t.Fatal(err)
	}
	ss.checkCalls(t, []string{"Lock", "Unlock", "8"})

	err = p.Set(10)
	if err != fErr {
		t.Fatal(err)
	}
	ss.checkCalls(t, []string{"Lock", "Unlock", "10"})

	// New value should have been set regardless of the error
	if p.Get() != 10 {
		t.Fatal(p)
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
		t.Fatal(ss.l)
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
