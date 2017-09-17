package util_test

import (
	"testing"
	"time"

	"github.com/divtxt/raft/util"
)

func TestTriggeredRunner(t *testing.T) {
	fRunning := false
	fRunCount := 0
	f := func() {
		fRunning = true
		time.Sleep(20 * time.Millisecond)
		fRunning = false
		fRunCount++
	}

	tr := util.NewTriggeredRunner(f)
	if fRunning || fRunCount != 0 {
		t.Fatal()
	}

	// No first run without trigger
	time.Sleep(10 * time.Millisecond)
	if fRunning || fRunCount != 0 {
		t.Fatal()
	}

	// TriggerRun should run the function
	tr.TriggerRun() // Run #1
	time.Sleep(10 * time.Millisecond)
	if !fRunning {
		t.Fatal()
	}
	time.Sleep(20 * time.Millisecond)
	if fRunCount != 1 {
		t.Fatal()
	}

	// Extra TriggerRuns should return immediately and collapse into 1 run
	tr.TriggerRun() // Run #2
	time.Sleep(10 * time.Millisecond)
	if !fRunning {
		t.Fatal()
	}
	tr.TriggerRun() // Run #3
	tr.TriggerRun() // should be collapsed into pending run #3
	tr.TriggerRun() // should be collapsed into pending run #3
	time.Sleep(20 * time.Millisecond)
	if fRunCount != 2 {
		t.Fatal()
	}
	if !fRunning {
		t.Fatal()
	}
	time.Sleep(20 * time.Millisecond)
	if fRunning || fRunCount != 3 {
		t.Fatal()
	}

	// StopSync waits for current run to complete
	tr.TriggerRun() // Run #4
	time.Sleep(10 * time.Millisecond)
	if !fRunning {
		t.Fatal()
	}
	tr.StopSync()
	if fRunning || fRunCount != 4 {
		t.Fatal()
	}

	// TriggerRun & StopSync should now panic
	err := util.ExpectPanicMessage(tr.TriggerRun, "send on closed channel")
	if err != nil {
		t.Fatal(err)
	}
	err = util.ExpectPanicMessage(tr.StopSync, "close of closed channel")
	if err != nil {
		t.Fatal(err)
	}

	// TestHelperFakeRestart & TestHelperRunOnceIfTriggerPending
	tr.TestHelperFakeRestart()
	if tr.TestHelperRunOnceIfTriggerPending() {
		t.Fatal()
	}
	tr.TriggerRun() // Run #5
	time.Sleep(10 * time.Millisecond)
	if fRunning || fRunCount != 4 {
		t.Fatal()
	}
	if !tr.TestHelperRunOnceIfTriggerPending() {
		t.Fatal()
	}
	if fRunning || fRunCount != 5 {
		t.Fatal()
	}
}
