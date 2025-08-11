// Package ctrl provides a set of control functions for assertions, error handling, HTTP server management,
// and graceful shutdown handling in Go applications. Built for Go 1.21+, it offers a clean API with flexible
// configuration options and no external runtime dependencies.
//
// # Assertions
//
// The package provides assertion functions that panic when conditions are not met, useful for runtime
// invariant checking:
//
//	ctrl.Assert(user.IsAuthenticated())
//	ctrl.Assertf(count > 0, "expected positive count, got %d", count)
//	ctrl.AssertFunc(func() bool { return database.IsConnected() })
//	ctrl.AssertFuncf(func() bool { return cache.Size() < maxSize }, "cache exceeded: %d", cache.Size())
//
// # Error Handling
//
// For scenarios where returning an error is more appropriate than panicking, the package provides
// ErrorOr variants:
//
//	if err := ctrl.ErrorOr(user.IsAuthenticated()); err != nil {
//	    return err
//	}
//	if err := ctrl.ErrorOrf(count > 0, "expected positive count, got %d", count); err != nil {
//	    return err
//	}
//	if err := ctrl.ErrorOrFunc(func() bool { return database.IsConnected() }); err != nil {
//	    return err
//	}
//	if err := ctrl.ErrorOrFuncf(func() bool { return cache.Size() < maxSize },
//	    "cache size exceeded: %d/%d", cache.Size(), maxSize); err != nil {
//	    return err
//	}
//	customErr := errors.New("database not connected")
//	if err := ctrl.ErrorOrWithErr(database.IsConnected(), customErr); err != nil {
//	    return err  // Returns customErr if condition fails
//	}
//	if err := ctrl.ErrorOrFuncWithErr(func() bool { return cache.Size() < maxSize }, ErrCacheFull); err != nil {
//	    return err  // Returns ErrCacheFull if condition fails
//	}
//
// # HTTP Server Management
//
// The package helps manage HTTP server lifecycle, particularly graceful shutdown:
//
//	// Shutdown an HTTP server with a timeout
//	err := ctrl.ShutdownHTTPServer(ctx, server,
//	    ctrl.WithHTTPShutdownTimeout(5*time.Second))
//
//	// Run a server with context-aware shutdown
//	errCh := ctrl.RunHTTPServerWithContext(ctx, server,
//	    func() error { return server.ListenAndServe() },
//	    ctrl.WithHTTPShutdownTimeout(5*time.Second),
//	    ctrl.WithHTTPLogger(logger))
//
// # Graceful Shutdown
//
// The package provides robust handling of process termination signals:
//
//	// Basic setup
//	ctx, cancel := ctrl.GracefulShutdown()
//	defer cancel()
//
//	// With custom configuration
//	ctx, cancel := ctrl.GracefulShutdown(
//	    ctrl.WithTimeout(30*time.Second),
//	    ctrl.WithSignals(syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP),
//	    ctrl.WithExitCode(2),
//	    ctrl.WithOnShutdown(func(sig os.Signal) {
//	        log.Printf("shutting down due to %s signal", sig)
//	        database.Close()
//	    }),
//	    ctrl.WithOnForceExit(func() {
//	        log.Printf("force exiting after timeout")
//	    }),
//	    ctrl.WithLogger(logger))
//
// # Best Practices
//
// Use assertions for internal invariants that should never fail in correct code:
//
//	ctrl.Assert(len(buffer) >= headerSize)  // Internal invariant
//
// Use ErrorOr variants for validating external input or recoverable conditions:
//
//	if err := ctrl.ErrorOr(len(userInput) < maxLength); err != nil {
//	    return err  // External input validation
//	}
//
// For HTTP servers, combine graceful shutdown with server lifecycle management:
//
//	ctx, cancel := ctrl.GracefulShutdown(ctrl.WithTimeout(10*time.Second))
//	defer cancel()
//	errCh := ctrl.RunHTTPServerWithContext(ctx, server, server.ListenAndServe)
//	if err := <-errCh; err != nil {
//	    log.Fatalf("server error: %v", err)
//	}
package ctrl
