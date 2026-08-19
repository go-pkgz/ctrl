package ctrl

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// HTTPOption represents a functional option for HTTP server operations.
type HTTPOption func(*httpOptions)

type httpOptions struct {
	shutdownTimeout time.Duration
	logger          *slog.Logger
}

// WithHTTPShutdownTimeout sets the maximum time to wait for server shutdown.
func WithHTTPShutdownTimeout(timeout time.Duration) HTTPOption {
	return func(o *httpOptions) {
		o.shutdownTimeout = timeout
	}
}

// WithHTTPLogger sets a custom logger for HTTP server operations.
func WithHTTPLogger(logger *slog.Logger) HTTPOption {
	return func(o *httpOptions) {
		o.logger = logger
	}
}

// ShutdownHTTPServer gracefully shuts down an HTTP server with a timeout.
// It returns any error encountered during shutdown.
func ShutdownHTTPServer(ctx context.Context, server *http.Server, opts ...HTTPOption) error {
	options := httpOptions{
		shutdownTimeout: 10 * time.Second, // default shutdown timeout
	}

	for _, opt := range opts {
		opt(&options)
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, options.shutdownTimeout)
	defer cancel()

	return server.Shutdown(shutdownCtx)
}

// RunHTTPServerWithContext runs a server start function and ensures it shuts down gracefully
// when the provided context is canceled.
// The startFn is responsible for starting the server (e.g., ListenAndServe).
// It returns a channel that will receive any error from the server.
//
// When the result is published depends on how the server stops. On context cancellation it comes
// after the graceful shutdown drained the connections, so the caller may terminate the process at
// that point; if the shutdown timeout expires first, the shutdown error is published as soon as the
// timeout is reported, requests may well still be running, and startFn is not waited for, so a
// serving error surfacing after that point is not reported. The error can be inspected with
// errors.Is(err, context.DeadlineExceeded). A server that fails on its own reports right away and is
// left untouched, so the caller can start it again. Connections that net/http does not wait for,
// hijacked ones and those dropped by Server.Close among them, remain the caller's to track, as do
// the callbacks of Server.RegisterOnShutdown, which Shutdown starts without awaiting them.
func RunHTTPServerWithContext(ctx context.Context, server *http.Server, startFn func() error, opts ...HTTPOption) <-chan error {
	options := httpOptions{
		shutdownTimeout: 10 * time.Second, // default timeout
		logger:          slog.Default(),
	}

	for _, opt := range opts {
		opt(&options)
	}

	// channel to report the final result to the caller
	errCh := make(chan error, 1)

	// serveCh collects the result of startFn, always exactly one value
	serveCh := make(chan error, 1)
	go func() { serveCh <- startFn() }()

	// single coordinator owning errCh, it publishes only after the server stopped
	go func() {
		defer close(errCh)

		select {
		case err := <-serveCh:
			// the server gave up on its own, the caller learns why immediately and keeps the
			// server intact, so a failed start can be retried on it
			errCh <- serveResult(err)
			return
		case <-ctx.Done():
			// both can be ready at once, and a server that already stopped is not shut down
			select {
			case err := <-serveCh:
				errCh <- serveResult(err)
				return
			default:
			}
		}

		options.logger.Info("shutting down HTTP server")

		// the parent context is already canceled, so the shutdown gets its own deadline while
		// keeping the context values
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), options.shutdownTimeout)
		defer cancel()

		shutdownErr := server.Shutdown(shutdownCtx)
		if shutdownErr != nil {
			options.logger.Error("server shutdown error", "error", shutdownErr)
			shutdownErr = fmt.Errorf("shutdown http server: %w", shutdownErr)
		}

		var serveErr error
		if shutdownErr == nil {
			// the drain is over, startFn returns as soon as the listeners are closed
			serveErr = serveResult(<-serveCh)
		} else {
			// the drain already gave up, waiting on startFn on top of that would defeat the timeout
			select {
			case err := <-serveCh:
				serveErr = serveResult(err)
			default:
			}
		}

		switch {
		case serveErr != nil && shutdownErr != nil:
			errCh <- errors.Join(serveErr, shutdownErr)
		case serveErr != nil:
			errCh <- serveErr
		default:
			errCh <- shutdownErr
		}
	}()

	return errCh
}

// serveResult converts the result of the server start function to the value reported to the caller,
// dropping the expected ErrServerClosed produced by a graceful shutdown.
func serveResult(err error) error {
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
