package ctrl

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// GracefulShutdown handles process termination with graceful shutdown.
// It returns a context that is canceled when a termination signal is received
// and a cancel function that can be called to trigger shutdown manually.
func GracefulShutdown(opts ...ShutdownOption) (context.Context, context.CancelFunc) {
	config := shutdownConfig{
		signals:     []os.Signal{os.Interrupt, syscall.SIGTERM},
		timeout:     10 * time.Second,
		forceExit:   true,
		exitCode:    1,
		onShutdown:  func(_ os.Signal) {},
		onForceExit: func() {},
		logger:      slog.Default(),
		osExit:      os.Exit,
	}

	for _, opt := range opts {
		opt(&config)
	}

	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, config.signals...)

	go func() {
		sig := <-sigChan
		config.logger.Warn("received signal, shutting down...", "signal", sig)
		config.onShutdown(sig)
		cancel() // trigger graceful shutdown

		if !config.forceExit {
			return
		}

		// wait for timeout or second signal
		select {
		case <-time.After(config.timeout):
			config.logger.Warn("forced exit after timeout", "timeout", config.timeout)
			config.onForceExit()
			config.osExit(config.exitCode)
		case sig := <-sigChan:
			config.logger.Warn("received second signal, forcing exit", "signal", sig)
			config.onForceExit()
			config.osExit(config.exitCode)
		}
	}()

	return ctx, cancel
}

// ShutdownOption configures shutdown behavior
type ShutdownOption func(*shutdownConfig)

type shutdownConfig struct {
	signals     []os.Signal
	timeout     time.Duration
	forceExit   bool
	exitCode    int
	onShutdown  func(os.Signal)
	onForceExit func()
	logger      *slog.Logger
	osExit      func(int) // for testing to avoid actual os.Exit
}

// WithSignals sets which signals trigger the shutdown
func WithSignals(signals ...os.Signal) ShutdownOption {
	return func(c *shutdownConfig) {
		c.signals = signals
	}
}

// WithTimeout sets the maximum time to wait for graceful shutdown
func WithTimeout(timeout time.Duration) ShutdownOption {
	return func(c *shutdownConfig) {
		c.timeout = timeout
	}
}

// WithoutForceExit disables the forced exit after timeout
func WithoutForceExit() ShutdownOption {
	return func(c *shutdownConfig) {
		c.forceExit = false
	}
}

// WithExitCode sets the exit code used for forced exits
func WithExitCode(code int) ShutdownOption {
	return func(c *shutdownConfig) {
		c.exitCode = code
	}
}

// WithOnShutdown sets a callback function that is called when shutdown begins
func WithOnShutdown(fn func(os.Signal)) ShutdownOption {
	return func(c *shutdownConfig) {
		c.onShutdown = fn
	}
}

// WithOnForceExit sets a callback function that is called right before forced exit
func WithOnForceExit(fn func()) ShutdownOption {
	return func(c *shutdownConfig) {
		c.onForceExit = fn
	}
}

// WithLogger sets a custom slog.Logger for shutdown messages
func WithLogger(logger *slog.Logger) ShutdownOption {
	return func(c *shutdownConfig) {
		c.logger = logger
	}
}

// withOsExit is for testing only - allows overriding os.Exit
func withOsExit(exit func(int)) ShutdownOption { //nolint:unused // false positive, used in tests
	return func(c *shutdownConfig) {
		c.osExit = exit
	}
}
