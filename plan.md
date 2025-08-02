# Ferret Enhancement Implementation Plan

## Overview
This plan outlines the implementation of the enhancements described in `enhancements.md` to transform Ferret from a proof-of-concept into a production-ready HTTP instrumentation library. The implementation will be done in phases to ensure backward compatibility and incremental improvements.

## Progress Summary
- **Phase 1**: ✅ COMPLETED (2025-08-02) - Core safety and architecture improvements
- **Phase 2**: ⏳ Not started - Enhanced metrics collection
- **Phase 3**: ⏳ Not started - Observability integration
- **Phase 4**: ⏳ Not started - Testing and quality
- **Phase 5**: ⏳ Not started - CLI tool enhancement
- **Phase 6**: ⏳ Not started - Documentation and release

## Phase 1: Core Safety and Architecture (Priority: Critical) ✅ COMPLETED

### 1.1 Thread-Safe Result Storage ✅
**Goal**: Make Ferret safe for concurrent use by removing global mutable state.

**Tasks**:
- [x] Create `Result` struct to hold all per-request timing data
- [x] Implement context-based result storage using `context.WithValue`
- [x] Refactor `Ferret` struct to be immutable during requests
- [x] Add result retrieval function `GetResult(req *http.Request) *Result`

**Files modified**:
- `pkg/ferret/ferret.go`: Complete refactor of timing storage
- `pkg/ferret/result.go`: Created with Result struct and methods

### 1.2 Migrate to DialContext ✅
**Goal**: Use modern context-aware dialing for proper cancellation support.

**Tasks**:
- [x] Replace deprecated `Dial` with `DialContext`
- [x] Ensure context propagation throughout the request lifecycle
- [x] Add proper error handling for context cancellation

**Files modified**:
- `pkg/ferret/ferret.go`: Updated transport configuration

### 1.3 Functional Options Pattern ✅
**Goal**: Create flexible configuration without breaking changes.

**Tasks**:
- [x] Define `Option` type and constructor `New(opts ...Option)`
- [x] Create options for:
  - [x] `WithKeepAlives(enabled bool)`
  - [x] `WithTimeout(connect, total time.Duration)`
  - [x] `WithTransport(base http.RoundTripper)`
  - [x] `WithClock(func() time.Time)` for testing
  - [x] `WithDialer(dialer *net.Dialer)`
  - [x] `WithTLSHandshakeTimeout(timeout time.Duration)`

**Files created**:
- `pkg/ferret/options.go`: Option definitions and implementations

### 1.4 Testing ✅
**Additional work completed**:
- [x] Created comprehensive test suite (`pkg/ferret/ferret_test.go`)
- [x] Verified thread safety with race detector
- [x] Tested all options and configurations
- [x] Tested Result methods and JSON serialization
- [x] Maintained backward compatibility with legacy API

## Phase 2: Enhanced Metrics Collection (Priority: High)

### 2.1 HTTPTrace Integration
**Goal**: Capture detailed timing breakdowns (DNS, TLS, first-byte).

**Tasks**:
- [ ] Extend `Result` struct with new timing fields
- [ ] Implement `httptrace.ClientTrace` hooks
- [ ] Calculate phase durations (DNS time, TLS time, etc.)
- [ ] Ensure trace integration with context

**Files to modify**:
- `pkg/ferret/ferret.go`: Add httptrace implementation
- `pkg/ferret/result.go`: New file for Result struct and methods

### 2.2 Result Serialization
**Goal**: Enable easy consumption of metrics.

**Tasks**:
- [ ] Implement `(*Result).MarshalJSON()` for JSON output
- [ ] Add `(*Result).String()` for human-readable format
- [ ] Create phase duration calculation methods

**Files to create**:
- `pkg/ferret/result.go`: Result methods and serialization

## Phase 3: Observability Integration (Priority: Medium)

### 3.1 Prometheus Support
**Goal**: First-class metrics export for production monitoring.

**Tasks**:
- [ ] Create `WithPrometheus(collectors)` option
- [ ] Implement histogram collection for each phase
- [ ] Add labels for method, host, status code
- [ ] Document Prometheus integration patterns

**Files to create**:
- `pkg/ferret/prometheus.go`: Prometheus-specific integration

### 3.2 OpenTelemetry Support (Optional)
**Goal**: Support modern tracing standards.

**Tasks**:
- [ ] Create `WithOpenTelemetry(tracer)` option
- [ ] Implement span creation and attribute setting
- [ ] Ensure proper span relationships

**Files to create**:
- `pkg/ferret/otel.go`: OpenTelemetry integration

## Phase 4: Testing and Quality (Priority: High)

### 4.1 Unit Tests
**Goal**: Comprehensive test coverage for all functionality.

**Tasks**:
- [ ] Test concurrent usage (race conditions)
- [ ] Test context cancellation
- [ ] Test all timing calculations
- [ ] Test with `httptest.Server`
- [ ] Mock time for deterministic tests

**Files to create**:
- `pkg/ferret/ferret_test.go`: Core functionality tests
- `pkg/ferret/result_test.go`: Result serialization tests
- `pkg/ferret/options_test.go`: Configuration tests

### 4.2 Integration Tests
**Goal**: Validate real-world scenarios.

**Tasks**:
- [ ] Test with various HTTP servers
- [ ] Test with connection failures
- [ ] Test with timeouts
- [ ] Benchmark performance impact

**Files to create**:
- `pkg/ferret/integration_test.go`: Real-world scenario tests

### 4.3 Examples
**Goal**: Demonstrate usage patterns.

**Tasks**:
- [ ] Basic usage example
- [ ] Prometheus integration example
- [ ] CLI tool example
- [ ] Concurrent usage example

**Files to create**:
- `examples/basic/main.go`
- `examples/prometheus/main.go`
- `examples/cli/main.go`

## Phase 5: CLI Tool Enhancement (Priority: Low)

### 5.1 Ferret CLI Tool
**Goal**: Standalone command-line tool for quick latency checks.

**Tasks**:
- [ ] Extract AWS testing logic to separate package
- [ ] Create general-purpose CLI using new Ferret library
- [ ] Add JSON output support
- [ ] Add configurable iterations and concurrency

**Files to create**:
- `cmd/ferret/main.go`: New CLI implementation
- `internal/aws/regions.go`: AWS-specific logic

## Phase 6: Documentation and Release (Priority: Medium)

### 6.1 Documentation
**Tasks**:
- [ ] Update README with new features
- [ ] Add API documentation
- [ ] Create migration guide from v1
- [ ] Add performance comparison

### 6.2 CI/CD Setup
**Tasks**:
- [ ] Create GitHub Actions workflow
- [ ] Add linting (golangci-lint)
- [ ] Add test coverage reporting
- [ ] Add release automation

**Files to create**:
- `.github/workflows/ci.yml`
- `.golangci.yml`

### 6.3 Release Preparation
**Tasks**:
- [ ] Add semantic versioning tags
- [ ] Create CHANGELOG.md
- [ ] Update go.mod module path if needed
- [ ] Ensure MIT license compatibility

## Implementation Order

1. **Week 1**: Phase 1 (Core Safety) - Critical for any production use
2. **Week 2**: Phase 2 (Enhanced Metrics) + Phase 4.1 (Unit Tests)
3. **Week 3**: Phase 3 (Observability) + Phase 4.2-4.3 (Integration/Examples)
4. **Week 4**: Phase 5 (CLI) + Phase 6 (Documentation/Release)

## Backward Compatibility Strategy

- Keep original `main.go` functionality intact during development
- Use v2 module path if breaking changes are necessary
- Provide migration guide for existing users
- Tag last v1 version before major changes

## Success Criteria

- [ ] Race detector passes all tests
- [ ] Concurrent usage benchmark shows no performance degradation
- [ ] At least 80% test coverage
- [ ] Examples run successfully
- [ ] Documentation is clear and comprehensive
- [ ] Can be used as drop-in replacement for timing needs