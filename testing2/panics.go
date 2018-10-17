package testing2

import (
	"fmt"
	"testing"
)

// AssertPanicsWith calls the given function and checks that it panics with the given value.
func AssertPanicsWith(t *testing.T, f func(), expectedRecover interface{}) {
	panicked := true
	defer func() {
		if panicked {
			r := recover()
			if r != expectedRecover {
				t.Fatalf("Expected panic: %v; got: %v", expectedRecover, r)
			}
		} else {
			t.Fatalf("Expected panic: %v; got nothing!", expectedRecover)
		}
	}()

	f()
	panicked = false
}

// AssertPanicsWithString calls the given function and checks that it panics with
// a value that formats to the given string.
func AssertPanicsWithString(t *testing.T, f func(), expectedRecover string) {
	panicked := true
	defer func() {
		if panicked {
			r := recover()
			if fmt.Sprintf("%v", r) != expectedRecover {
				t.Errorf("Expected panic: %v; got: %v", expectedRecover, r)
			}
		} else {
			t.Fatalf("Expected panic: %v; got nothing!", expectedRecover)
		}
	}()

	f()
	panicked = false
}
