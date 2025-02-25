package ctrl

import "fmt"

// Assert panics if the condition is false.
func Assert(condition bool) {
	if !condition {
		panic("assertion failed")
	}
}

// Assertf panics if the condition is false, with a formatted message.
func Assertf(condition bool, format string, args ...any) {
	if !condition {
		m := fmt.Sprintf(format, args...)
		panic("assertion failed: " + m)
	}
}

// AssertFunc panics if the function returns false.
func AssertFunc(f func() bool) {
	if !f() {
		panic("assertion failed")
	}
}

// AssertFuncf panics if the function returns false, with a formatted message.
func AssertFuncf(f func() bool, format string, args ...any) {
	if !f() {
		m := fmt.Sprintf(format, args...)
		panic("assertion failed: " + m)
	}
}
