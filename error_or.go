package ctrl

import (
	"fmt"
)

// ErrorOr returns nil if condition is true, otherwise returns an error.
func ErrorOr(condition bool) error {
	if !condition {
		return fmt.Errorf("assertion failed")
	}
	return nil
}

// ErrorOrf returns nil if condition is true, otherwise returns an error with a formatted message.
func ErrorOrf(condition bool, format string, args ...any) error {
	if !condition {
		m := fmt.Sprintf(format, args...)
		return fmt.Errorf("assertion failed: %s", m)
	}
	return nil
}

// ErrorOrFunc returns nil if the function returns true, otherwise returns an error.
func ErrorOrFunc(f func() bool) error {
	if !f() {
		return fmt.Errorf("assertion failed")
	}
	return nil
}

// ErrorOrFuncf returns nil if the function returns true, otherwise returns an error with a formatted message.
func ErrorOrFuncf(f func() bool, format string, args ...any) error {
	if !f() {
		m := fmt.Sprintf(format, args...)
		return fmt.Errorf("assertion failed: %s", m)
	}
	return nil
}

// ErrorOrWithErr returns nil if condition is true, otherwise returns the given error.
func ErrorOrWithErr(condition bool, err error) error {
	if !condition {
		return err
	}
	return nil
}

// ErrorOrFuncWithErr returns nil if the function returns true, otherwise returns the given error.
func ErrorOrFuncWithErr(f func() bool, err error) error {
	if !f() {
		return err
	}
	return nil
}
