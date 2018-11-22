package util_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/divtxt/raft/util"
)

func TestTicker(t *testing.T) {
	var n int32 = 0
	getN := func() int32 { return atomic.LoadInt32(&n) }
	incN := func() { atomic.AddInt32(&n, 1) }

	foo := func() {
		incN()
	}
	tick := 20 * time.Millisecond

	ticker := util.NewTicker(foo, tick)

	if getN() != 0 {
		t.Fatal(getN())
	}

	time.Sleep(tick + tick/2)
	if getN() != 1 {
		t.Fatal(getN())
	}

	time.Sleep(tick)
	if getN() != 2 {
		t.Fatal(getN())
	}

	time.Sleep(tick)
	if getN() != 3 {
		t.Fatal(getN())
	}

	ticker.StopAsync()

	time.Sleep(tick)
	if getN() != 3 {
		t.Fatal(getN())
	}
}
