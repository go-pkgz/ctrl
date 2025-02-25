package ctrl

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorOr(t *testing.T) {
	t.Run("ErrorOr", func(t *testing.T) {
		// should return nil when condition is true
		err := ErrorOr(true)
		assert.NoError(t, err)

		// should return error when condition is false
		err = ErrorOr(false)
		assert.Error(t, err)
		assert.Equal(t, "assertion failed", err.Error())
	})

	t.Run("ErrorOrf", func(t *testing.T) {
		// should return nil when condition is true
		err := ErrorOrf(true, "custom message")
		assert.NoError(t, err)

		// should return formatted error when condition is false
		msg := "test message"
		err = ErrorOrf(false, msg)
		assert.Error(t, err)
		assert.Equal(t, "assertion failed: "+msg, err.Error())

		// should support formatting
		err = ErrorOrf(false, "value is %d", 42)
		assert.Error(t, err)
		assert.Equal(t, "assertion failed: value is 42", err.Error())
	})

	t.Run("ErrorOrFunc", func(t *testing.T) {
		// should return nil when function returns true
		err := ErrorOrFunc(func() bool { return true })
		assert.NoError(t, err)

		// should return error when function returns false
		err = ErrorOrFunc(func() bool { return false })
		assert.Error(t, err)
		assert.Equal(t, "assertion failed", err.Error())

		// should evaluate function
		counter := 0
		_ = ErrorOrFunc(func() bool {
			counter++
			return true
		})
		assert.Equal(t, 1, counter)
	})

	t.Run("ErrorOrFuncf", func(t *testing.T) {
		// should return nil when function returns true
		err := ErrorOrFuncf(func() bool { return true }, "custom message")
		assert.NoError(t, err)

		// should return formatted error when function returns false
		msg := "custom func message"
		err = ErrorOrFuncf(func() bool { return false }, msg)
		assert.Error(t, err)
		assert.Equal(t, "assertion failed: "+msg, err.Error())

		// should support formatting
		err = ErrorOrFuncf(func() bool { return false }, "value is %d", 42)
		assert.Error(t, err)
		assert.Equal(t, "assertion failed: value is 42", err.Error())
	})

	t.Run("WithCustomError", func(t *testing.T) {
		customErr := errors.New("custom error")

		// should return custom error when condition is false
		err := ErrorOrWithErr(false, customErr)
		assert.Equal(t, customErr, err)

		// should return nil when condition is true
		err = ErrorOrWithErr(true, customErr)
		assert.NoError(t, err)

		// should work with function variant
		err = ErrorOrFuncWithErr(func() bool { return false }, customErr)
		assert.Equal(t, customErr, err)

		err = ErrorOrFuncWithErr(func() bool { return true }, customErr)
		assert.NoError(t, err)
	})
}
