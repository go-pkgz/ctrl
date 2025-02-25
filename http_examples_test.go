package ctrl_test

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-pkgz/ctrl"
)

// Example_httpServerWithContext demonstrates how to run an HTTP server that shuts down
// gracefully when the parent context is canceled.
func Example_httpServerWithContext() {
	// create a context that we can cancel to trigger shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create a simple HTTP handler
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, World!")
	})

	// create server
	server := &http.Server{
		Addr:    "localhost:0", // random port
		Handler: mux,
	}

	// create a custom logger for the example
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	// start server with options
	errCh := ctrl.RunHTTPServerWithContext(
		ctx,
		server,
		func() error {
			return server.ListenAndServe()
		},
		ctrl.WithHTTPShutdownTimeout(15*time.Second),
		ctrl.WithHTTPLogger(logger),
	)

	// for example only - trigger shutdown after a brief delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		fmt.Println("Triggering shutdown...")
		cancel()
	}()

	// wait for server to exit and check for errors
	err := <-errCh
	if err != nil {
		fmt.Println("Server error:", err)
	} else {
		fmt.Println("Server stopped gracefully")
	}

	// Output:
	// Triggering shutdown...
	// Server stopped gracefully
}
