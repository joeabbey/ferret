# Ferret Project Guide for Claude

This document provides essential information for Claude instances working on the Ferret project.

## Project Overview

Ferret is a production-ready HTTP instrumentation library for Go that provides detailed timing metrics. It consists of:
- A thread-safe HTTP RoundTripper library (`pkg/ferret`)
- A CLI tool for latency testing (`cmd/ferret`)
- A legacy TUI for AWS region testing (root `main.go`)

## Common Commands

### Building and Testing
```bash
# Run tests with coverage
go test -v -race -coverprofile=coverage.out ./pkg/...

# Run benchmarks
go test -bench=. -benchmem ./pkg/ferret

# Build the library
go build -v ./pkg/ferret

# Build CLI tool
go build -v -o ferret-cli ./cmd/ferret

# Build legacy TUI
go build -v -o ferret-tui .
```

### Linting and Code Quality
```bash
# Run linter (golangci-lint v2.3.0) - ALWAYS run before committing
golangci-lint run

# Format code
gofmt -s -w .

# Check for vet issues
go vet ./...
```

### Git Workflow
```bash
# IMPORTANT: Always run golangci-lint before pushing changes
golangci-lint run

# Check PR status
gh pr view <number>

# View PR checks
gh pr checks <number>

# Create new PR
gh pr create --title "Title" --body "Description"
```

### Pre-Push Checklist
**ALWAYS complete these steps before pushing changes:**
1. Run `golangci-lint run` and fix any issues
2. Run tests: `go test -v -race ./pkg/...`
3. Verify formatting: `gofmt -s -w .`
4. Check for any uncommitted changes: `git status`

## High-Level Architecture

### Core Library (`pkg/ferret`)
The library implements an HTTP RoundTripper that wraps the standard library's transport to capture detailed timing metrics:

1. **ferret.go**: Main transport implementation
   - Thread-safe design using sync.Map for storing results
   - Implements http.RoundTripper interface
   - Captures DNS, TCP, TLS, and TTFB timings

2. **options.go**: Functional options pattern for configuration
   - Timeout configuration
   - Keep-alive settings
   - Custom transport/dialer support
   - Clock injection for testing

3. **result.go**: Timing result storage and calculations
   - Stores all timing phases
   - Provides duration calculation methods
   - Thread-safe access to timing data

4. **prometheus.go**: Prometheus metrics integration
   - Histogram metrics for request durations
   - Phase-specific timing metrics
   - Label support for method, host, status

5. **otel.go**: OpenTelemetry integration
   - Distributed tracing support
   - Span creation with timing attributes
   - Context propagation

### CLI Tool (`cmd/ferret`)
Modern CLI implementation with multiple modes:
- Single URL testing with detailed timing output
- AWS region selection mode
- Load testing with concurrency support
- JSON/plain text output formats

### Legacy TUI (`main.go`)
Terminal UI for visual AWS region latency testing:
- Concurrent testing of multiple regions
- Real-time visual updates
- Color-coded latency indicators

## Key Architectural Decisions

1. **Thread Safety**: The library uses sync.Map for storing results to ensure thread-safe access across concurrent requests.

2. **Zero Allocation Design**: Core timing logic avoids allocations in hot paths for optimal performance.

3. **Functional Options**: Configuration uses the functional options pattern for flexibility and backward compatibility.

4. **Context Integration**: Full support for Go's context package, including cancellation and deadline propagation.

5. **Minimal Dependencies**: Core library only depends on standard library, with optional integrations for Prometheus and OpenTelemetry.

## CI/CD Pipeline

The project uses GitHub Actions with the following jobs:

1. **Test**: Runs on Ubuntu, macOS, and Windows with Go 1.23
   - Race detection enabled
   - Coverage reporting to Codecov
   - Platform-specific handling for Windows

2. **Lint**: Uses golangci-lint v2.3.0
   - Runs on Ubuntu only
   - Strict configuration with multiple linters enabled

3. **Security**: Uses gosec for security scanning
   - Checks for common security issues
   - Runs on all code including examples

4. **Build**: Cross-platform builds
   - Builds library, CLI, and TUI
   - Uploads artifacts for each platform

5. **Benchmark**: Performance testing on Ubuntu
   - Ensures no performance regressions

## Common Issues and Solutions

1. **Windows Test Failures**: The CI uses conditional logic to handle Windows-specific test command formatting.

2. **Windows Timing Resolution**: Windows has lower timer resolution than Unix systems (typically 15.6ms vs 1ms). This can cause very fast operations to report zero duration, leading to test failures. Tests should:
   - Allow zero durations on Windows but not on other platforms
   - Check for negative durations (which should never happen)
   - Use `runtime.GOOS != "windows"` to apply platform-specific assertions
   - Example pattern:
   ```go
   duration := result.TotalDuration()
   if duration < 0 {
       t.Error("Duration should not be negative")
   } else if duration == 0 && runtime.GOOS != "windows" {
       t.Error("Expected positive duration")
   }
   ```

3. **Linter Configuration**: The project uses golangci-lint v2 with a minimal configuration due to v2's strict schema requirements.

4. **Deprecated APIs**: The codebase has been updated to avoid deprecated Go standard library functions (e.g., io/ioutil).

5. **Linter Failures in CI**: Always run `golangci-lint run` locally before pushing to avoid CI failures. The CI will reject PRs with linting issues.

## Code Style Guidelines

1. **Error Handling**: Explicitly handle or ignore errors using `_ =` pattern
2. **Comments**: Use standard Go comment format, avoid "DEPRECATED:" in favor of "Deprecated:"
3. **Line Length**: Keep lines under 130 characters where practical
4. **Imports**: Group standard library, external, and internal imports
5. **Testing**: Use table-driven tests where appropriate

## Testing Approach

1. **Unit Tests**: Comprehensive tests for all public APIs
2. **Integration Tests**: HTTP server tests with timing verification
3. **Benchmarks**: Performance benchmarks for critical paths
4. **Race Detection**: All tests run with -race flag in CI
5. **Coverage**: Target high coverage, report to Codecov

## Dependencies

- Go 1.23+ (uses toolchain directive for 1.23.11)
- External dependencies (core library):
  - github.com/prometheus/client_golang v1.19.1 (optional)
  - go.opentelemetry.io/otel v1.37.0 (optional)
- CLI dependencies:
  - github.com/gizak/termui/v3 v3.1.0 (for legacy TUI)

## Security Considerations

1. **No Secrets**: Never commit API keys or credentials
2. **TLS**: Minimum TLS 1.2 for all HTTPS connections
3. **Error Handling**: Avoid exposing sensitive information in errors
4. **HTTP Timeouts**: Always configure appropriate timeouts

## Future Considerations

1. The library is designed for extensibility through the functional options pattern
2. New timing phases can be added to the Result struct
3. Additional observability integrations can be added following the existing patterns
4. The CLI tool can be extended with new modes and output formats