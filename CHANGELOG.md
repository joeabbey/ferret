# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Thread-safe HTTP RoundTripper implementation
- Context-based result storage for concurrent safety
- Detailed timing metrics using `net/http/httptrace`
- Functional options pattern for flexible configuration
- Prometheus metrics integration
- OpenTelemetry tracing support
- New CLI tool with multiple output formats
- AWS region latency testing mode
- Comprehensive test suite with 78.6% coverage
- Benchmark tests showing minimal performance overhead
- GitHub Actions CI/CD pipeline
- golangci-lint configuration

### Changed
- Complete rewrite of core library for production use
- Migrated from global state to context-based storage
- Replaced deprecated `Dial` with `DialContext`
- Moved AWS testing logic to separate internal package
- Updated documentation with migration guide

### Fixed
- Race conditions in concurrent usage
- Context cancellation handling
- Memory leaks in long-running applications

### Security
- No longer stores sensitive timing data in global variables
- Proper context cancellation prevents resource leaks

## [1.0.0] - 2019-04-21

### Added
- Initial release
- TUI for AWS region latency testing
- Basic HTTP timing functionality
- Support for concurrent endpoint testing

[Unreleased]: https://github.com/joeabbey/ferret/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/joeabbey/ferret/releases/tag/v1.0.0