// file: ctrl/shutdown_test.go
package ctrl

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"sync"
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

// waitLimit bounds how long a test waits for something that should happen immediately.
// it is a failure deadline rather than a delay, a loaded machine simply waits less of it.
const waitLimit = 5 * time.Second

// awaitExit blocks until the injected exit function is called and returns the code it received
func (s *ShutdownTestSuite) awaitExit(calls <-chan int) int {
	s.T().Helper()
	select {
	case code := <-calls:
		return code
	case <-time.After(waitLimit):
		s.Fail("exit function was not called")
		return -1
	}
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
		case <-time.After(waitLimit):
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
		case <-time.After(waitLimit):
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
		case <-time.After(waitLimit):
			s.Fail("context not canceled after signal")
		}
	})

	s.Run("force exit", func() {
		exitCalls := make(chan int, 1)

		_, cancel := GracefulShutdown(
			WithTimeout(100*time.Millisecond),
			WithExitCode(42),
			withOsExit(func(code int) { exitCalls <- code }),
		)
		defer cancel()

		// trigger shutdown
		process, err := os.FindProcess(os.Getpid())
		s.NoError(err)
		s.NoError(process.Signal(os.Interrupt))

		s.Equal(42, s.awaitExit(exitCalls))
	})

	s.Run("second signal", func() {
		exitCalls := make(chan int, 1)

		// the timeout is long enough that only the second signal can cause the exit
		shutdownCtx, cancel := GracefulShutdown(
			WithTimeout(time.Hour),
			WithExitCode(2),
			withOsExit(func(code int) { exitCalls <- code }),
		)
		defer cancel()

		// send first signal to start shutdown
		process, err := os.FindProcess(os.Getpid())
		s.NoError(err)
		s.NoError(process.Signal(os.Interrupt))

		// the cancellation marks the first signal as processed
		select {
		case <-shutdownCtx.Done():
		case <-time.After(waitLimit):
			s.Fail("context not canceled after the first signal")
			return
		}

		s.NoError(process.Signal(os.Interrupt))
		s.Equal(2, s.awaitExit(exitCalls))
	})

	s.Run("on force exit callback", func() {
		forceExitCalls := make(chan struct{}, 1)

		_, cancel := GracefulShutdown(
			WithTimeout(50*time.Millisecond),
			WithOnForceExit(func() { forceExitCalls <- struct{}{} }),
			withOsExit(func(int) {}),
		)
		defer cancel()

		// send signal to trigger shutdown
		process, err := os.FindProcess(os.Getpid())
		s.NoError(err)
		s.NoError(process.Signal(os.Interrupt))

		select {
		case <-forceExitCalls:
		case <-time.After(waitLimit):
			s.Fail("force exit callback was not called")
		}
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

	s.Run("concurrent signals", func() {
		exitCalls := make(chan int, 4)

		// the case runs on a signal of its own, so that a straggler of the burst below cannot
		// reach another case. os/signal drops a send to a full channel, so a burst may well be
		// delivered once, and the exit is left to the timeout rather than to a second signal;
		// that a second signal forces the exit is what the case above covers
		_, cancel := GracefulShutdown(
			WithSignals(syscall.SIGUSR2),
			WithTimeout(100*time.Millisecond),
			withOsExit(func(code int) { exitCalls <- code }),
		)
		defer cancel()

		// send multiple signals concurrently
		process, err := os.FindProcess(os.Getpid())
		s.Require().NoError(err)

		var wg sync.WaitGroup
		send := func(sig os.Signal) {
			defer wg.Done()
			_ = process.Signal(sig)
		}
		wg.Add(4)
		for range 4 {
			go send(syscall.SIGUSR2)
		}

		code := s.awaitExit(exitCalls)
		wg.Wait()
		s.Equal(1, code)
	})

	s.Run("timeout accuracy", func() {
		timeout := 200 * time.Millisecond
		// the shutdown callback runs when the signal arrives, which is when the timer starts
		startCalls, exitCalls := make(chan time.Time, 1), make(chan time.Time, 1)

		// the case measures a timer, so it runs on a signal no other case sends
		_, cancel := GracefulShutdown(
			WithSignals(syscall.SIGUSR1),
			WithTimeout(timeout),
			WithOnShutdown(func(os.Signal) { startCalls <- time.Now() }),
			withOsExit(func(int) { exitCalls <- time.Now() }),
		)
		defer cancel()

		process, err := os.FindProcess(os.Getpid())
		s.NoError(err)
		s.NoError(process.Signal(syscall.SIGUSR1))

		var started, exited time.Time
		for _, c := range []struct {
			ch  chan time.Time
			dst *time.Time
			msg string
		}{{startCalls, &started, "shutdown callback was not called"}, {exitCalls, &exited, "forced exit was not called"}} {
			select {
			case *c.dst = <-c.ch:
			case <-time.After(waitLimit):
				s.Fail(c.msg)
				return
			}
		}

		// the forced exit must never happen early; the upper bound stays loose enough to
		// survive a delayed timer while still catching a timeout off by an order of magnitude
		elapsed := exited.Sub(started)
		s.GreaterOrEqual(elapsed, timeout)
		s.Less(elapsed, 10*timeout)
	})
}
