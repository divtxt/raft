package testhelpers

import (
	"fmt"
	"testing"
)

// TestHelper_ExpectPanic calls the given function and checks that it panics, and that the recover
// value is equal to the given value.
func TestHelper_ExpectPanic(t *testing.T, f func(), expectedRecover interface{}) {
	skipRecover := false
	defer func() {
		if !skipRecover {
			if r := recover(); r != expectedRecover {
				t.Fatal(fmt.Sprintf("Expected panic: %v; got: %v", expectedRecover, r))
			}
		}
	}()

	f()
	skipRecover = true
	t.Fatal(fmt.Sprintf("Expected panic: %v; got nothing!", expectedRecover))
}

// TestHelper_ExpectPanic calls the given function and checks that it panics, and that the recover
// value formatted as a string is equal to the given string.
func TestHelper_ExpectPanicMessage(t *testing.T, f func(), expectedRecover string) {
	skipRecover := false
	defer func() {
		if !skipRecover {
			if r := recover(); fmt.Sprintf("%v", r) != expectedRecover {
				t.Fatal(fmt.Sprintf("Expected panic: %v; got: %v", expectedRecover, r))
			}
		}
	}()

	f()
	skipRecover = true
	t.Fatal(fmt.Sprintf("Expected panic: %v; got nothing!", expectedRecover))
}
