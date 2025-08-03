# Ferret

A production-ready HTTP instrumentation library for Go that provides detailed timing metrics, with a powerful CLI tool for latency testing and AWS region selection.

## Features

- **Thread-safe HTTP RoundTripper**: Safe for concurrent use across goroutines
- **Detailed timing metrics**: DNS, TCP connection, TLS handshake, TTFB, and data transfer
- **Context support**: Full support for context cancellation and deadlines
- **Observability integrations**: Built-in Prometheus and OpenTelemetry support
- **Flexible configuration**: Functional options pattern for easy customization
- **Zero dependencies**: Core library has minimal external dependencies
- **CLI tool**: Powerful command-line tool for HTTP latency testing

## Installation

### Library

```bash
go get github.com/joeabbey/ferret/pkg/ferret
```

### CLI Tool

```bash
go install github.com/joeabbey/ferret/cmd/ferret@latest
```

## Quick Start

### Using the Library

```go
package main

import (
    "fmt"
    "net/http"
    "github.com/joeabbey/ferret/pkg/ferret"
)

func main() {
    // Create a new Ferret transport
    transport := ferret.New()
    client := &http.Client{Transport: transport}
    
    // Make a request
    resp, err := client.Get("https://example.com")
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()
    
    // Get timing information
    result := ferret.GetResult(resp.Request)
    if result != nil {
        fmt.Printf("Total time: %v\n", result.TotalDuration())
        fmt.Printf("DNS lookup: %v\n", result.DNSDuration())
        fmt.Printf("TCP connection: %v\n", result.ConnectionDuration())
        fmt.Printf("TLS handshake: %v\n", result.TLSDuration())
        fmt.Printf("Time to first byte: %v\n", result.TTFB())
    }
}
```

### Using the CLI

```bash
# Test a single URL
ferret -url https://example.com

# Find the fastest AWS region
ferret -mode aws

# JSON output for automation
ferret -url https://api.example.com -format json

# Concurrent load testing
ferret -url https://api.example.com -iterations 100 -concurrency 10
```

## Advanced Usage

### Configuration Options

```go
transport := ferret.New(
    ferret.WithTimeout(5*time.Second, 30*time.Second),
    ferret.WithKeepAlives(false),
    ferret.WithTLSHandshakeTimeout(10*time.Second),
)
```

### Prometheus Integration

```go
// Create Prometheus collectors
histogramVec := prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
        Name: "http_request_duration_seconds",
        Help: "HTTP request latency distributions.",
    },
    []string{"method", "host", "status"},
)

// Configure Ferret with Prometheus
transport := ferret.New(
    ferret.WithPrometheus(&ferret.PrometheusConfig{
        DurationHistogram: histogramVec,
    }),
)
```

### OpenTelemetry Integration

```go
// Configure Ferret with OpenTelemetry
transport := ferret.New(
    ferret.WithOpenTelemetry(tracer, ferret.DefaultSpanNameFormatter),
)
```

## Performance

Ferret is designed to have minimal performance impact:

- **Low overhead**: Typically adds <1ms to request time
- **Zero allocations**: In hot paths after initialization
- **Thread-safe**: Safe for use in high-concurrency scenarios

Benchmark results (MacBook Pro M1):
```
BenchmarkFerret-8         500000      2584 ns/op      0 B/op      0 allocs/op
BenchmarkBaseline-8       500000      2341 ns/op      0 B/op      0 allocs/op
```

## API Documentation

### Core Types

#### `Ferret`
The main transport type that implements `http.RoundTripper`.

#### `Result`
Contains all timing information for a request:
- `Start`, `End`: Request start and end times
- `DNSStart`, `DNSDone`: DNS lookup timing
- `ConnectStart`, `ConnectDone`: TCP connection timing
- `TLSHandshakeStart`, `TLSHandshakeDone`: TLS timing
- `FirstByte`: Time to first byte received
- `Error`: Any error that occurred

### Functions

#### `New(opts ...Option) *Ferret`
Creates a new Ferret transport with the given options.

#### `GetResult(req *http.Request) *Result`
Retrieves timing information from a request. Must be called on the response's request object.

### Options

- `WithTransport(rt http.RoundTripper)`: Set base transport
- `WithTimeout(connectTimeout, totalTimeout time.Duration)`: Configure timeouts
- `WithKeepAlives(enabled bool)`: Enable/disable keep-alives
- `WithTLSHandshakeTimeout(timeout time.Duration)`: Set TLS timeout
- `WithPrometheus(config *PrometheusConfig)`: Enable Prometheus metrics
- `WithOpenTelemetry(tracer trace.Tracer, spanNameFormatter func(*http.Request) string)`: Enable OpenTelemetry

## Migration from v1

If you're using the original Ferret AWS latency testing tool:

1. The TUI functionality has been moved to a separate legacy binary
2. The new CLI tool provides the same AWS testing capability with more features:
   ```bash
   # Old way (TUI)
   ferret
   
   # New way (CLI)
   ferret -mode aws
   ```

3. The library API has changed to be thread-safe:
   ```go
   // Old (not thread-safe)
   f := ferret.NewFerret()
   f.ConnectStart = time.Now() // Direct field access
   
   // New (thread-safe)
   f := ferret.New()
   result := ferret.GetResult(resp.Request)
   connectDuration := result.ConnectionDuration()
   ```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see LICENSE file for details.