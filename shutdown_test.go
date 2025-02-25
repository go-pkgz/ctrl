package ctrl

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type ShutdownTestSuite struct {
	suite.Suite
}

func TestShutdownSuite(t *testing.T) {
	suite.Run(t, new(ShutdownTestSuite))
}

func (s *ShutdownTestSuite) TestGracefulShutdown() {
	s.Run("context is canceled on signal", func() {
		// capture logs for verification
		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, nil))

		shutdownCtx, cancel := GracefulShutdown(
			WithLogger(logger),
			WithoutForceExit(),
		)
		defer cancel()

		// simulate a signal to trigger shutdown
		process, err := os.FindProcess(os.Getpid())
		s.NoError(err)
		s.NoError(process.Signal(os.Interrupt))

		// wait for context cancellation or timeout
		select {
		case <-shutdownCtx.Done():
			// this is what we expect
			s.Equal(context.Canceled, shutdownCtx.Err())
		case <-time.After(500 * time.Millisecond):
			s.Fail("context was not canceled within timeout")
		}

		// verify log message
		s.Contains(buf.String(), "received signal")
		s.Contains(buf.String(), "interrupt")
	})

	s.Run("callbacks are invoked", func() {
		var shutdownCalled bool
		var signalReceived os.Signal

		shutdownCtx, cancel := GracefulShutdown(
			WithOnShutdown(func(sig os.Signal) {
				shutdownCalled = true
				signalReceived = sig
			}),
			WithoutForceExit(),
		)
		defer cancel()

		// simulate signal
		process, err := os.FindProcess(os.Getpid())
		s.NoError(err)
		s.NoError(process.Signal(os.Interrupt))

		// wait for context to be canceled
		select {
		case <-shutdownCtx.Done():
			// expected
		case <-time.After(500 * time.Millisecond):
			s.Fail("context not canceled within timeout")
		}

		s.True(shutdownCalled)
		s.Equal(os.Interrupt, signalReceived)
	})

	s.Run("custom signals", func() {
		shutdownCtx, cancel := GracefulShutdown(
			WithSignals(syscall.SIGUSR1),
			WithoutForceExit(),
		)
		defer cancel()

		// send sigusr1 which we're listening for
		process, err := os.FindProcess(os.Getpid())
		s.NoError(err)
		s.NoError(process.Signal(syscall.SIGUSR1))

		// wait for context to be canceled
		select {
		case <-shutdownCtx.Done():
			// expected
		case <-time.After(500 * time.Millisecond):
			s.Fail("context not canceled after signal")
		}
	})

	s.Run("force exit", func() {
		exitCode := -1
		exitCalled := false

		mockExit := func(code int) {
			exitCalled = true
			exitCode = code
		}

		_, cancel := GracefulShutdown(
			WithTimeout(100*time.Millisecond),
			WithExitCode(42),
			withOsExit(mockExit),
		)
		defer cancel()

		// trigger shutdown
		process, err := os.FindProcess(os.Getpid())
		s.NoError(err)
		s.NoError(process.Signal(os.Interrupt))

		// wait for mock exit to be called
		time.Sleep(200 * time.Millisecond)
		s.True(exitCalled)
		s.Equal(42, exitCode)
	})

	s.Run("second signal", func() {
		exitCalled := false
		exitCode := -1

		mockExit := func(code int) {
			exitCalled = true
			exitCode = code
		}

		_, cancel := GracefulShutdown(
			WithTimeout(500*time.Millisecond),
			WithExitCode(2),
			withOsExit(mockExit),
		)
		defer cancel()

		// send first signal to start shutdown
		process, err := os.FindProcess(os.Getpid())
		s.NoError(err)
		s.NoError(process.Signal(os.Interrupt))

		// wait a bit and then send a second signal
		time.Sleep(50 * time.Millisecond)
		s.NoError(process.Signal(os.Interrupt))

		// verify exit was called
		time.Sleep(50 * time.Millisecond)
		s.True(exitCalled)
		s.Equal(2, exitCode)
	})

	s.Run("on force exit callback", func() {
		forceExitCalled := false
		mockExit := func(int) {}

		_, cancel := GracefulShutdown(
			WithTimeout(50*time.Millisecond),
			WithOnForceExit(func() {
				forceExitCalled = true
			}),
			withOsExit(mockExit),
		)
		defer cancel()

		// send signal to trigger shutdown
		process, err := os.FindProcess(os.Getpid())
		s.NoError(err)
		s.NoError(process.Signal(os.Interrupt))

		// wait for timeout and force exit
		time.Sleep(100 * time.Millisecond)
		s.True(forceExitCalled)
	})

	s.Run("manual cancel", func() {
		shutdownCtx, cancel := GracefulShutdown(
			WithoutForceExit(),
		)

		// call cancel directly
		cancel()

		// context should be canceled
		s.Equal(context.Canceled, shutdownCtx.Err())
	})
}
