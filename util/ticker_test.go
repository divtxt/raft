package util_test

import (
	util "github.com/divtxt/raft/util"
	"sync/atomic"
	"testing"
	"time"
)

func TestTicker(t *testing.T) {
	var n int32 = 0

	getN := func() int32 {
		return atomic.LoadInt32(&n)
	}

	f := func() {
		atomic.AddInt32(&n, 1)
	}

	ticker := util.NewTicker(f, 10*time.Millisecond)

	if getN() != 0 {
		t.Fatal(getN())
	}

	time.Sleep(15 * time.Millisecond)
	if getN() != 1 {
		t.Fatal(getN())
	}

	time.Sleep(10 * time.Millisecond)
	if getN() != 2 {
		t.Fatal(getN())
	}

	time.Sleep(10 * time.Millisecond)
	if getN() != 3 {
		t.Fatal(getN())
	}

	ticker.StopSync()

	time.Sleep(10 * time.Millisecond)
	if getN() != 3 {
		t.Fatal(getN())
	}
}
