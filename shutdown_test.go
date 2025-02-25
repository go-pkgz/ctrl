// file: ctrl/shutdown_test.go
package ctrl

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"sync/atomic"
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
		var shutdownCalled int32
		var signalReceived atomic.Value

		shutdownCtx, cancel := GracefulShutdown(
			WithOnShutdown(func(sig os.Signal) {
				atomic.StoreInt32(&shutdownCalled, 1)
				signalReceived.Store(sig)
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

		s.Equal(int32(1), atomic.LoadInt32(&shutdownCalled))
		s.Equal(os.Interrupt, signalReceived.Load())
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
		var exitCode int32 = -1
		var exitCalled int32

		mockExit := func(code int) {
			atomic.StoreInt32(&exitCalled, 1)
			atomic.StoreInt32(&exitCode, int32(code))
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
		s.Equal(int32(1), atomic.LoadInt32(&exitCalled))
		s.Equal(int32(42), atomic.LoadInt32(&exitCode))
	})

	s.Run("second signal", func() {
		var exitCalled int32
		var exitCode int32 = -1

		mockExit := func(code int) {
			atomic.StoreInt32(&exitCalled, 1)
			atomic.StoreInt32(&exitCode, int32(code))
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
		s.Equal(int32(1), atomic.LoadInt32(&exitCalled))
		s.Equal(int32(2), atomic.LoadInt32(&exitCode))
	})

	s.Run("on force exit callback", func() {
		var forceExitCalled int32
		mockExit := func(int) {}

		_, cancel := GracefulShutdown(
			WithTimeout(50*time.Millisecond),
			WithOnForceExit(func() {
				atomic.StoreInt32(&forceExitCalled, 1)
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
		s.Equal(int32(1), atomic.LoadInt32(&forceExitCalled))
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
