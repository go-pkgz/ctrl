package ctrl

import (
	"context"
	"errors"
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
func RunHTTPServerWithContext(ctx context.Context, server *http.Server, startFn func() error, opts ...HTTPOption) <-chan error {
	options := httpOptions{
		shutdownTimeout: 10 * time.Second, // default timeout
		logger:          slog.Default(),
	}

	for _, opt := range opts {
		opt(&options)
	}

	// channel to report server errors
	errCh := make(chan error, 1)

	// start server
	go func() {
		err := startFn()

		// only report non-shutdown errors
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		} else {
			errCh <- nil
		}
		close(errCh)
	}()

	// handle graceful shutdown
	go func() {
		<-ctx.Done()

		options.logger.Info("shutting down HTTP server")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), options.shutdownTimeout)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil { //nolint:contextcheck // context non-inherited intentionally
			options.logger.Error("server shutdown error", "error", err)
		}
	}()

	return errCh
}
