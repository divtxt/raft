package util_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/divtxt/raft/testing2"
	"github.com/divtxt/raft/util"
)

func TestStoppableGoroutine_FuncEnds(t *testing.T) {
	var n int32 = 0
	getN := func() int32 { return atomic.LoadInt32(&n) }

	f := func(<-chan struct{}) {
		atomic.AddInt32(&n, 1)
	}

	sg := util.StartGoroutine(f)

	// Allow time for goroutine to run
	time.Sleep(10 * time.Millisecond)

	if !sg.Stopped() {
		t.Fatal()
	}
	if getN() != 1 {
		t.Fatal(getN())
	}
}

func TestStoppableGoroutine_Join(t *testing.T) {
	foo := func(<-chan struct{}) {
		time.Sleep(10 * time.Millisecond)
	}

	sg := util.StartGoroutine(foo)

	sg.Join()
	if !sg.Stopped() {
		t.Fatal()
	}

	// Should be safe to call StopAsync once even if goroutine has stopped on it's own
	sg.StopAsync()

	// Should be safe to call Join more than once.
	sg.Join()
}

func TestStoppableGoroutine_LongRunningFunc(t *testing.T) {
	tick := 20 * time.Millisecond

	foo := func(stop <-chan struct{}) {
		<-stop
		time.Sleep(tick)
	}

	g := util.StartGoroutine(foo)

	// Long running function blocks
	time.Sleep(2 * tick)
	if g.Stopped() {
		t.Fatal()
	}

	// Stop can take some time
	g.StopAsync()
	time.Sleep(tick / 2)
	if g.Stopped() {
		t.Fatal()
	}
	time.Sleep(tick)
	if !g.Stopped() {
		t.Fatal()
	}

	// Extra call to StopAsync should panic
	testing2.AssertPanicsWithString(t, g.StopAsync, "close of closed channel")
}

func TestStoppableGoroutine_StopSync(t *testing.T) {
	foo := func(stop <-chan struct{}) {
		<-stop
	}

	g := util.StartGoroutine(foo)

	// Long running function blocks
	time.Sleep(20 * time.Millisecond)
	if g.Stopped() {
		t.Fatal()
	}

	// StopSync
	g.StopSync()
	if !g.Stopped() {
		t.Fatal()
	}

	// Extra call to StopSync should panic
	testing2.AssertPanicsWithString(t, g.StopSync, "close of closed channel")
}
