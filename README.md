# ctrl [![Build Status](https://github.com/go-pkgz/ctrl/workflows/build/badge.svg)](https://github.com/go-pkgz/ctrl/actions) [![Coverage Status](https://coveralls.io/repos/github/go-pkgz/ctrl/badge.svg?branch=master)](https://coveralls.io/github/go-pkgz/ctrl?branch=master) [![godoc](https://godoc.org/github.com/go-pkgz/ctrl?status.svg)](https://godoc.org/github.com/go-pkgz/ctrl)

`ctrl` provides a set of control functions for assertions, HTTP server management, and graceful shutdown handling in Go applications. Built for Go 1.21+, it offers a clean API with flexible configuration options.

## Features

- Runtime assertions with optional formatted messages
- HTTP server lifecycle management
- Graceful shutdown with signal handling
- Context-based cancellation
- Configurable timeouts and callbacks
- Comprehensive error handling
- No external dependencies except for testing

## Quick Start

Here's a practical example showing how to implement graceful shutdown in an HTTP server:

```go
func main() {
    // Set up graceful shutdown
    ctx, cancel := ctrl.GracefulShutdown(
        ctrl.WithTimeout(10*time.Second),
        ctrl.WithOnShutdown(func(sig os.Signal) {
            log.Printf("shutdown initiated by signal: %v", sig)
        }),
    )
    defer cancel()

    // Create a simple HTTP server
    mux := http.NewServeMux()
    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintln(w, "Hello, World!")
    })

    server := &http.Server{
        Addr:    ":8080",
        Handler: mux,
    }

    // Start server with context-aware shutdown
    errCh := ctrl.RunHTTPServerWithContext(
        ctx,
        server,
        func() error {
            return server.ListenAndServe()
        },
        ctrl.WithHTTPShutdownTimeout(5*time.Second),
    )

    // Wait for server to exit and check for errors
    if err := <-errCh; err != nil {
        log.Fatalf("Server error: %v", err)
    }
}
```

## Core Concepts

### Assertions

The package provides assertion functions that panic when conditions are not met, useful for runtime invariant checking:

```go
// Simple assertion
ctrl.Assert(user.IsAuthenticated())

// Formatted assertion
ctrl.Assertf(count > 0, "expected positive count, got %d", count)

// Function-based assertions
ctrl.AssertFunc(func() bool {
    return database.IsConnected()
})

ctrl.AssertFuncf(func() bool {
    return cache.Size() < maxSize
}, "cache size exceeded: %d/%d", cache.Size(), maxSize)
```

### HTTP Server Management

The package helps manage HTTP server lifecycle, particularly graceful shutdown:

```go
// Shutdown an HTTP server with a timeout
err := ctrl.ShutdownHTTPServer(ctx, server, 
    ctrl.WithHTTPShutdownTimeout(5*time.Second))

// Run a server with context-aware shutdown
errCh := ctrl.RunHTTPServerWithContext(ctx, server, 
    func() error { 
        return server.ListenAndServe() 
    },
    ctrl.WithHTTPShutdownTimeout(5*time.Second),
    ctrl.WithHTTPLogger(logger),
)
```

### Graceful Shutdown

The package provides a robust way to handle process termination signals:

```go
// Basic setup
ctx, cancel := ctrl.GracefulShutdown()
defer cancel()

// With custom configuration
ctx, cancel := ctrl.GracefulShutdown(
    ctrl.WithTimeout(30*time.Second),
    ctrl.WithSignals(syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP),
    ctrl.WithExitCode(2),
    ctrl.WithOnShutdown(func(sig os.Signal) {
        log.Printf("shutting down due to %s signal", sig)
        database.Close()
    }),
    ctrl.WithOnForceExit(func() {
        log.Printf("force exiting after timeout")
    }),
    ctrl.WithLogger(logger),
)
```

## Install and update

```bash
go get -u github.com/go-pkgz/ctrl
```

## Usage Examples

### Assertion Usage

```go
func processItems(items []Item) {
    // Ensure we have items to process
    ctrl.Assertf(len(items) > 0, "no items to process")
    
    for _, item := range items {
        // Ensure each item is valid
        ctrl.Assert(item.IsValid())
        
        // Process the item
        process(item)
    }
}
```

### HTTP Server with Graceful Shutdown

```go
func startServer() error {
    // Set up graceful shutdown
    ctx, cancel := ctrl.GracefulShutdown(
        ctrl.WithTimeout(10*time.Second),
        ctrl.WithLogger(logger),
    )
    defer cancel()
    
    // Create server
    server := &http.Server{
        Addr:    ":8080",
        Handler: router,
    }
    
    // Run server
    errCh := ctrl.RunHTTPServerWithContext(ctx, server, server.ListenAndServe)
    
    // Wait for shutdown
    return <-errCh
}
```

### Multi-Stage Shutdown

```go
func main() {
    // Create shutdown context
    ctx, cancel := ctrl.GracefulShutdown(
        ctrl.WithOnShutdown(func(sig os.Signal) {
            log.Printf("shutdown sequence initiated by %v", sig)
        }),
    )
    defer cancel()
    
    // Create multiple services with the same shutdown context
    httpServer := setupHTTPServer(ctx)
    grpcServer := setupGRPCServer(ctx)
    cacheService := setupCache(ctx)
    
    // Wait for context cancellation (shutdown signal)
    <-ctx.Done()
    
    // Context was canceled, services will be shutting down
    log.Println("waiting for all services to complete shutdown")
    
    // Additional cleanup if needed
    database.Close()
}
```

## Optional Parameters

The package uses functional options pattern for configuration:

### HTTP Server Options

```go
// WithHTTPShutdownTimeout sets the maximum time to wait for server shutdown
WithHTTPShutdownTimeout(timeout time.Duration)

// WithHTTPLogger sets a custom logger for HTTP server operations
WithHTTPLogger(logger *slog.Logger)
```

### Graceful Shutdown Options

```go
// WithSignals sets which signals trigger the shutdown
WithSignals(signals ...os.Signal)

// WithTimeout sets the maximum time to wait for graceful shutdown
WithTimeout(timeout time.Duration)

// WithoutForceExit disables the forced exit after timeout
WithoutForceExit()

// WithExitCode sets the exit code used for forced exits
WithExitCode(code int)

// WithOnShutdown sets a callback function called when shutdown begins
WithOnShutdown(fn func(os.Signal))

// WithOnForceExit sets a callback function called right before forced exit
WithOnForceExit(fn func())

// WithLogger sets a custom logger for shutdown messages
WithLogger(logger *slog.Logger)
```

## Best Practices

1. **Assertions**: Use for internal invariants, not user input validation
   ```go
   // Good: internal invariant
   ctrl.Assert(len(buffer) >= headerSize)
   
   // Bad: user input validation
   ctrl.Assert(len(userInput) < maxLength) // Don't do this
   ```

2. **Graceful Shutdown**: Allow sufficient time for connections to close
   ```go
   // Shorter timeout for development
   ctrl.WithTimeout(5*time.Second)
   
   // Longer timeout for production with many connections
   ctrl.WithTimeout(30*time.Second)
   ```

3. **HTTP Server Context**: Create a separate context for each server
   ```go
   // Each server gets its own timeout and configuration
   apiErrCh := ctrl.RunHTTPServerWithContext(ctx, apiServer, apiServer.ListenAndServe,
       ctrl.WithHTTPShutdownTimeout(10*time.Second))
   
   adminErrCh := ctrl.RunHTTPServerWithContext(ctx, adminServer, adminServer.ListenAndServe,
       ctrl.WithHTTPShutdownTimeout(5*time.Second))
   ```

4. **Shutdown Callbacks**: Use for resource cleanup
   ```go
   ctrl.WithOnShutdown(func(sig os.Signal) {
       // Close database connections
       db.Close()
       
       // Flush logs
       logger.Sync()
       
       // Release other resources
       cache.Clear()
   })
   ```

## Error Handling

The package provides clear error handling patterns:

```go
// HTTP server shutdown
if err := ctrl.ShutdownHTTPServer(ctx, server); err != nil {
    log.Printf("error during server shutdown: %v", err)
}

// Running HTTP server with context
errCh := ctrl.RunHTTPServerWithContext(ctx, server, server.ListenAndServe)
if err := <-errCh; err != nil {
    log.Fatalf("server error: %v", err)
}
```

## ErrorOr Functions

The package provides variants of assertion functions that return errors instead of panicking. These are useful for validations where you want to return an error to the caller rather than crash the program:

```go
// Basic condition checking
if err := ctrl.ErrorOr(user.IsAuthenticated()); err != nil {
    return err
}

// With formatted error message
if err := ctrl.ErrorOrf(count > 0, "expected positive count, got %d", count); err != nil {
    return err
}

// Function-based variants
if err := ctrl.ErrorOrFunc(func() bool {
    return database.IsConnected()
}); err != nil {
    return err
}

// With custom error
customErr := ErrDatabaseNotConnected
if err := ctrl.ErrorOrWithErr(database.IsConnected(), customErr); err != nil {
    return err  // Will return customErr if condition fails
}
```

These functions are particularly useful in validators, middleware, and other scenarios where returning an error is more appropriate than panicking.


## Contributing

Contributions to `ctrl` are welcome! Please submit a pull request or open an issue for any bugs or feature requests.

## License

`ctrl` is available under the MIT license. See the [LICENSE](LICENSE) file for more info.