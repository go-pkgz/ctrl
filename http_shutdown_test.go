package ctrl

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShutdownHTTPServer(t *testing.T) {
	// create a test server that we can control
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	}

	// find an available port
	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	server.Addr = listener.Addr().String()

	// start the server, the listener already accepts connections so no wait is needed
	go func() {
		serveErr := server.Serve(listener)
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			t.Errorf("unexpected server error: %v", serveErr)
		}
	}()

	// verify server is running by making a request, bounded so a stuck server fails the test
	// rather than hanging it
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://" + server.Addr)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// test shutdown with default timeout
	err = ShutdownHTTPServer(context.Background(), server)
	require.NoError(t, err)

	// server should now be shut down, trying to connect should fail
	_, err = http.Get("http://" + server.Addr)
	assert.Error(t, err)
}

func TestRunHTTPServerWithContext(t *testing.T) {
	t.Run("successful server", func(t *testing.T) {
		// create a test server
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		server := &http.Server{
			Handler: mux,
		}

		// find an available port
		listener, err := net.Listen("tcp", "localhost:0")
		require.NoError(t, err)
		server.Addr = listener.Addr().String()

		// create a buffer to capture logs
		var logBuf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuf, nil))

		// create a cancelable context
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// create a custom start function that uses our listener
		startFn := func() error {
			return server.Serve(listener)
		}

		// run server with context, the listener already accepts connections so no wait is needed
		errCh := RunHTTPServerWithContext(ctx, server, startFn,
			WithHTTPLogger(logger),
			WithHTTPShutdownTimeout(3*time.Second),
		)

		// verify server is running by making a request, bounded so a stuck server fails the test
		// rather than hanging it
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get("http://" + server.Addr)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()

		// trigger shutdown
		cancel()

		// wait for server to exit
		err = <-errCh
		require.NoError(t, err)

		// server should now be shut down, trying to connect should fail
		_, err = http.Get("http://" + server.Addr)
		require.Error(t, err)

		// verify shutdown log message was recorded
		assert.Contains(t, logBuf.String(), "shutting down HTTP server")
	})

	t.Run("server error", func(t *testing.T) {
		// create a server with a deliberately invalid address
		server := &http.Server{
			Addr: "invalid-address",
		}

		// create a context
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// create a start function that will fail
		startFn := func() error {
			return server.ListenAndServe()
		}

		// run server with context
		errCh := RunHTTPServerWithContext(ctx, server, startFn)

		// wait for error
		err := <-errCh
		assert.Error(t, err)
	})

	t.Run("cancel before server starts", func(t *testing.T) {
		// create a test server
		server := &http.Server{
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
		}

		// create a context that's already canceled
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		// create a listener but don't actually use it in the start function
		listener, err := net.Listen("tcp", "localhost:0")
		require.NoError(t, err)
		defer listener.Close()
		server.Addr = listener.Addr().String()

		// create a start function that blocks until context is canceled
		startFn := func() error {
			<-ctx.Done()
			return http.ErrServerClosed
		}

		// run server with context
		errCh := RunHTTPServerWithContext(ctx, server, startFn)

		// wait for result
		err = <-errCh
		require.NoError(t, err)
	})

	t.Run("result delayed until in-flight request drains", func(t *testing.T) {
		handlerStarted, releaseHandler := make(chan struct{}), make(chan struct{})
		var handlerDone atomic.Bool

		server := &http.Server{
			Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				close(handlerStarted)
				<-releaseHandler
				handlerDone.Store(true)
				w.WriteHeader(http.StatusOK)
			}),
		}

		listener, err := net.Listen("tcp", "localhost:0")
		require.NoError(t, err)
		server.Addr = listener.Addr().String()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		errCh := RunHTTPServerWithContext(ctx, server, func() error { return server.Serve(listener) },
			WithHTTPLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
			WithHTTPShutdownTimeout(5*time.Second),
		)

		type result struct {
			status int
			err    error
		}
		respCh := make(chan result, 1)
		client := &http.Client{Timeout: 30 * time.Second} // bound so a stuck server fails the test
		go func() {
			resp, reqErr := client.Get("http://" + server.Addr)
			if reqErr != nil {
				respCh <- result{err: reqErr}
				return
			}
			defer resp.Body.Close()
			respCh <- result{status: resp.StatusCode}
		}()

		// request is in flight, ask for shutdown while the handler is still running
		<-handlerStarted
		cancel()

		// the caller must not be told the server is done while the handler is still working
		select {
		case err := <-errCh:
			t.Fatalf("result published before the handler finished: %v", err)
		case <-time.After(100 * time.Millisecond):
		}
		assert.False(t, handlerDone.Load(), "handler must still be in flight")

		close(releaseHandler)

		res := <-respCh
		require.NoError(t, res.err)
		assert.Equal(t, http.StatusOK, res.status, "in-flight request must complete")

		require.NoError(t, <-errCh)
		assert.True(t, handlerDone.Load())
	})

	t.Run("server left usable after a failed start", func(t *testing.T) {
		// occupy a port so that the first bind fails
		busy, err := net.Listen("tcp", "localhost:0")
		require.NoError(t, err)
		defer busy.Close()

		server := &http.Server{
			Addr: busy.Addr().String(),
			Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
		}
		discard := WithHTTPLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		require.Error(t, <-RunHTTPServerWithContext(ctx, server, server.ListenAndServe, discard))

		// the same server must start again, a failed start may not shut it down
		listener, err := net.Listen("tcp", "localhost:0")
		require.NoError(t, err)
		server.Addr = listener.Addr().String()

		errCh := RunHTTPServerWithContext(ctx, server, func() error { return server.Serve(listener) }, discard)

		client := &http.Client{Timeout: 30 * time.Second} // bound so a stuck server fails the test
		resp, err := client.Get("http://" + server.Addr)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		cancel()
		require.NoError(t, <-errCh)
	})

	t.Run("shutdown timeout reported to the caller", func(t *testing.T) {
		handlerStarted, releaseHandler := make(chan struct{}), make(chan struct{})
		requestDone := make(chan struct{})

		server := &http.Server{
			Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				close(handlerStarted)
				<-releaseHandler
				w.WriteHeader(http.StatusOK)
			}),
		}
		defer server.Close()

		listener, err := net.Listen("tcp", "localhost:0")
		require.NoError(t, err)
		server.Addr = listener.Addr().String()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var logBuf bytes.Buffer
		errCh := RunHTTPServerWithContext(ctx, server, func() error { return server.Serve(listener) },
			WithHTTPLogger(slog.New(slog.NewTextHandler(&logBuf, nil))),
			WithHTTPShutdownTimeout(100*time.Millisecond),
		)

		client := &http.Client{Timeout: 30 * time.Second} // bound so a stuck server fails the test
		go func() {
			defer close(requestDone)
			if resp, reqErr := client.Get("http://" + server.Addr); reqErr == nil {
				resp.Body.Close()
			}
		}()

		// the handler never returns before the shutdown timeout expires
		<-handlerStarted
		cancel()

		err = <-errCh
		require.Error(t, err, "a shutdown that times out must not report success")
		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Contains(t, logBuf.String(), "server shutdown error")

		close(releaseHandler)
		<-requestDone
	})
}
