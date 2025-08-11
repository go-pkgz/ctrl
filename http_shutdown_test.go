package ctrl

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
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

	// start the server
	go func() {
		serveErr := server.Serve(listener)
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			t.Errorf("unexpected server error: %v", serveErr)
		}
	}()

	// let the server start
	time.Sleep(100 * time.Millisecond)

	// verify server is running by making a request
	resp, err := http.Get("http://" + server.Addr)
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

		// run server with context
		errCh := RunHTTPServerWithContext(ctx, server, startFn,
			WithHTTPLogger(logger),
			WithHTTPShutdownTimeout(3*time.Second),
		)

		// let the server start
		time.Sleep(100 * time.Millisecond)

		// verify server is running by making a request
		resp, err := http.Get("http://" + server.Addr)
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
}
