package util

import (
	"fmt"
)

// ExpectPanic calls the given function and checks that it panics with the given value.
func ExpectPanic(f func(), expectedRecover interface{}) (err error) {
	tooLate := false
	defer func() {
		if !tooLate {
			if r := recover(); r != expectedRecover {
				err = fmt.Errorf("Expected panic: %v; got: %v", expectedRecover, r)
			}
		}
	}()

	f()
	tooLate = true
	return fmt.Errorf("Expected panic: %v; got nothing!", expectedRecover)
}

// ExpectPanicMessage calls the given function and checks that it panics with
// a value that formats to the given string.
func ExpectPanicMessage(f func(), expectedRecover string) (err error) {
	tooLate := false
	defer func() {
		if !tooLate {
			if r := recover(); fmt.Sprintf("%v", r) != expectedRecover {
				err = fmt.Errorf("Expected panic: %v; got: %v", expectedRecover, r)
			}
		}
	}()

	f()
	tooLate = true
	return fmt.Errorf("Expected panic: %v; got nothing!", expectedRecover)
}
