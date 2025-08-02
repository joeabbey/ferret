# Ferret CLI

A command-line tool for HTTP latency testing and AWS region selection.

## Installation

```bash
go install github.com/joeabbey/ferret/cmd/ferret@latest
```

## Usage

### Simple Mode

Test a single URL:

```bash
# Basic test
ferret -url https://example.com

# With custom iterations and concurrency
ferret -url https://example.com -iterations 20 -concurrency 5

# JSON output
ferret -url https://example.com -format json

# Short format (one line)
ferret -url https://example.com -format short

# Show detailed timing breakdown
ferret -url https://example.com -details
```

### AWS Mode

Find the fastest AWS region:

```bash
# Test all AWS regions
ferret -mode aws

# With custom iterations
ferret -mode aws -iterations 5

# JSON output for parsing
ferret -mode aws -format json
```

## Output Formats

### Text Format (default)
Shows detailed statistics including min, max, average, median, p90, and p99 latencies.

### JSON Format
Outputs machine-readable JSON for integration with other tools.

### Short Format
Single-line output ideal for scripts and monitoring.

## Options

- `-url`: URL to test (required for simple mode)
- `-mode`: Operating mode: "simple" or "aws" (default: simple)
- `-iterations`: Number of requests to make (default: 10)
- `-concurrency`: Number of concurrent requests (default: 1)
- `-format`: Output format: "text", "json", or "short" (default: text)
- `-timeout`: Request timeout (default: 30s)
- `-method`: HTTP method (default: GET)
- `-details`: Show detailed timing breakdown in text format

## Examples

```bash
# Quick latency check
ferret -url https://api.example.com -iterations 5 -format short

# Load test with concurrency
ferret -url https://api.example.com -iterations 100 -concurrency 10

# Find best AWS region for your location
ferret -mode aws

# Export results as JSON
ferret -url https://api.example.com -format json > results.json
```