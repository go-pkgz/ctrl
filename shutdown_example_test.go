package ctrl_test

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/go-pkgz/ctrl"
)

// Example_gracefulShutdown demonstrates basic usage of the GracefulShutdown function.
func Example_gracefulShutdown() {
	// normally you would use slog.Default(), but for the example we'll create a no-op logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// set up graceful shutdown
	_, cancel := ctrl.GracefulShutdown(
		ctrl.WithLogger(logger),
		ctrl.WithTimeout(5*time.Second),
	)
	defer cancel()

	fmt.Println("Application is running")
	fmt.Println("When SIGINT or SIGTERM is received, shutdown will be initiated")

	fmt.Println("Example complete (no actual signal sent)")

	// Output:
	// Application is running
	// When SIGINT or SIGTERM is received, shutdown will be initiated
	// Example complete (no actual signal sent)
}

// Example_gracefulShutdownWithCallbacks demonstrates using callbacks during shutdown.
func Example_gracefulShutdownWithCallbacks() {
	// for testing examples
	exampleDone := make(chan struct{})

	_, cancel := ctrl.GracefulShutdown(
		ctrl.WithLogger(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))),
		ctrl.WithTimeout(3*time.Second),
		ctrl.WithOnShutdown(func(sig os.Signal) {
			fmt.Printf("Shutdown initiated by signal: %v\n", sig)
			fmt.Println("Closing database connections...")
			time.Sleep(10 * time.Millisecond) // Simulate work
		}),
		ctrl.WithOnForceExit(func() {
			fmt.Println("Forced shutdown - cleanup incomplete!")
		}),
	)
	defer cancel()

	fmt.Println("Application running with shutdown callbacks configured")

	// for the example only, we'll manually cancel the context
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel() // Simulate a shutdown signal
		fmt.Println("Simulated shutdown signal")
		close(exampleDone)
	}()

	// for the example to complete
	<-exampleDone

	fmt.Println("Context canceled, starting graceful shutdown")

	// Output:
	// Application running with shutdown callbacks configured
	// Simulated shutdown signal
	// Context canceled, starting graceful shutdown
}

// Example_gracefulShutdownCustomConfiguration demonstrates custom shutdown configuration.
func Example_gracefulShutdownCustomConfiguration() {
	// create a noop logger for the example
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// set up graceful shutdown with several options
	_, cancel := ctrl.GracefulShutdown(
		ctrl.WithLogger(logger),
		ctrl.WithTimeout(3*time.Second),
		ctrl.WithExitCode(2),           // Non-zero exit code
		ctrl.WithoutForceExit(),        // Disable forced exit
		ctrl.WithSignals(os.Interrupt), // Only listen for Ctrl+C, not SIGTERM
	)
	defer cancel()

	fmt.Println("Application running with custom shutdown configuration")
	fmt.Println("- 3 second timeout")
	fmt.Println("- Exit code 2")
	fmt.Println("- Forced exit disabled")
	fmt.Println("- Only listening for SIGINT")

	// for the example to complete
	fmt.Println("Example complete (no signal sent)")

	// Output:
	// Application running with custom shutdown configuration
	// - 3 second timeout
	// - Exit code 2
	// - Forced exit disabled
	// - Only listening for SIGINT
	// Example complete (no signal sent)
}
