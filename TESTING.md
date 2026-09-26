# Testing Guide - RSS.Chat Go

Comprehensive guide to running all tests in the RSS.Chat Go repository, including the new WebSub protocol test suite.

## Quick Start

```bash
# Run all tests (full suite)
go test ./...

# Run all tests with race detector (recommended before commit)
go test -race ./...

# Run all tests with verbose output
go test -v ./...

# Run all tests with coverage
go test -cover ./...
```

## Complete Test Suite Overview

The project includes **150+ tests** across 12 packages:

| Package | Tests | Focus | Time |
|---------|-------|-------|------|
| `api/` | 45+ | HTTP endpoints, auth, headers | 2-5s |
| `websub/` | 38 | WebSub protocol, hub pinging | 4-5s |
| `websocket/` | 10+ | Real-time updates, broadcasting | 0.8s |
| `db/` | 20+ | Database operations, CRUD | 1-2s |
| `feed/` | 15+ | RSS/OPML generation | 1-2s |
| `config/` | 15+ | Configuration loading, validation | 1-2s |
| `setup/` | 10+ | Interactive setup, bootstrap | 1-2s |
| `publish/` | 8+ | Feed publishing, storage modes | 1-2s |
| `client/` | 5+ | Vendor client tests | 1-2s |
| `tools/backup/` | 8+ | Backup/restore operations | 1-2s |
| `tools/bluesky-subscribe/` | 15+ | Bluesky integration | 0.3s |
| `tools/restore/` | 2 | Restore operations | 0.2s |
| **Total** | **150+** | **All subsystems** | **~30s** |

## Running Tests by Category

### 1. Full Test Suite

Run everything (recommended before pushing):

```bash
# Standard run
go test ./...

# With race detector (catches concurrent bugs)
go test -race ./...

# Verbose output (see each test)
go test -v ./...

# With coverage report
go test -cover ./...

# Generate coverage HTML report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### 2. WebSub Tests Only

The new comprehensive WebSub test suite:

```bash
# All WebSub protocol tests (38 tests)
go test ./websub -v

# WebSub protocol tests with race detector
go test -race ./websub

# WebSub integration tests (14 tests)
go test ./api -run WebSub -v

# Both protocol and integration tests
go test -race ./websub ./api

# Specific WebSub test category
go test ./websub -run "FeedURL" -v     # URL encoding tests
go test ./websub -run "Concurrent" -v  # Concurrency tests
go test ./websub -run "Error" -v       # Error handling tests
go test ./api -run "Ping" -v           # Ping integration tests
go test ./api -run "Header" -v         # Header tests
```

### 3. API Tests

HTTP endpoints and handlers:

```bash
# All API tests
go test ./api -v

# Authentication tests
go test ./api -run "Auth" -v

# Feed endpoint tests
go test ./api -run "Feed" -v

# Feed tests (user, global, comments, OPML)
go test ./api -run "FeedFormat" -v

# Media upload tests
go test ./api -run "Media" -v

# Blocklist tests
go test ./api -run "Blocklist" -v

# WebSub tests
go test ./api -run "WebSub" -v
```

### 4. Database Tests

Data layer operations:

```bash
# All database tests
go test ./db -v

# CRUD operations
go test ./db -run "CRUD" -v

# Transaction tests
go test ./db -run "Transact" -v

# Blocklist database tests
go test ./db -run "Blocklist" -v
```

### 5. Feed Generation Tests

RSS/Atom/OPML generation:

```bash
# All feed tests
go test ./feed -v

# RSS feed generation
go test ./feed -run "RSS" -v

# OPML generation
go test ./feed -run "OPML" -v

# Markdown tests
go test ./feed -run "Markdown" -v

# Linkify tests
go test ./feed -run "Linkify" -v
```

### 6. Configuration Tests

Config loading and setup:

```bash
# All config tests
go test ./config -v

# Interactive setup tests
go test ./setup -v

# Config loading and defaults
go test ./config -run "Load|Default" -v

# Config validation
go test ./config -run "Validate" -v
```

### 7. Real-Time & WebSocket Tests

Live updates:

```bash
# WebSocket tests
go test ./websocket -v

# Hub subscription tests
go test ./websocket -run "Subscription" -v

# Broadcasting tests
go test ./websocket -run "Broadcast" -v

# Concurrency tests
go test ./websocket -run "Concurrent" -v
```

### 8. Tool Tests

CLI tools and utilities:

```bash
# Backup/restore tests
go test ./tools/backup -v
go test ./tools/restore -v

# Bluesky integration tests
go test ./tools/bluesky-subscribe -v
```

## Running Specific Tests

### Run a Single Test

```bash
# Exact test name
go test ./api -run TestNewPostEndpoint -v

# Test prefix matching
go test ./api -run "TestNew" -v  # Matches TestNewPostEndpoint, TestNewUser*, etc.
```

### Run Tests Matching Pattern

```bash
# Tests containing "Post" in the name
go test ./api -run "Post" -v

# Tests containing "Feed" OR "OPML"
go test ./feed -run "Feed|OPML" -v

# Case-sensitive matching
go test ./websub -run "Concurrent" -v
```

### Run Tests with Timeout

```bash
# Set 5 minute timeout per test
go test -timeout 5m ./...

# Disable timeout (careful!)
go test -timeout 0 ./...
```

## Debugging Tests

### Verbose Output

```bash
# Show all log output
go test -v ./...

# Show only failed tests
go test -v ./... 2>&1 | grep FAIL
```

### Failed Test Details

```bash
# Run one failing test with full output
go test -run TestNameThatFailed -v

# Keep test output even on success
go test -run TestName -v
```

### Race Detector Details

```bash
# Run with race detector (verbose)
go test -race -v ./...

# Shows goroutine creation traces on race detection
```

### Memory/Allocation Profiling

```bash
# Show memory allocations
go test -memprofile=mem.out ./api
go tool pprof mem.out

# Show CPU profile
go test -cpuprofile=cpu.out ./api
go tool pprof cpu.out
```

## Performance & Benchmarks

### Run Benchmarks

```bash
# Run all benchmarks in package
go test -bench=. ./db

# Run specific benchmark
go test -bench=BenchmarkQueryUser ./db

# With detailed stats
go test -bench=. -benchmem ./db

# Long benchmark run (e.g., 10 seconds per benchmark)
go test -bench=. -benchtime=10s ./db
```

### Test Timing

```bash
# Show test execution time
go test -v ./... | grep -E "RUN|PASS|FAIL|--- PASS|--- FAIL"

# Only slow tests (> 100ms)
go test -v ./... 2>&1 | awk '/Test.*[0-9]\.[0-9]+s/ && $NF ~ /^[0-9]\.[0-9]+s$/ && $NF > 0.1 {print}'
```

## Coverage Analysis

### Generate Coverage Report

```bash
# Generate coverage profile
go test -coverprofile=coverage.out ./...

# View coverage in terminal
go tool cover -func=coverage.out

# Generate HTML report
go tool cover -html=coverage.out

# View in browser
open coverage.html  # macOS
xdg-open coverage.html  # Linux
start coverage.html  # Windows
```

### Coverage by Package

```bash
# Show coverage for each package
for pkg in ./api ./db ./feed ./websub; do
  echo "=== $pkg ==="
  go test -coverprofile=coverage.out $pkg
  go tool cover -func=coverage.out | tail -1
done
```

## CI/CD & Pre-Commit Checks

### Pre-Commit Checklist

Run this before committing:

```bash
# Full build
go build ./...

# Run all tests with race detector
go test -race ./...

# Check formatting
gofmt -l .

# Run linter (if available)
go vet ./...

# Check for imports that can be removed
go mod tidy
```

### CI Pipeline Commands

Equivalent to `.github/workflows/ci.yml`:

```bash
# Build check
go build ./...

# Format check
gofmt -l .  # Must be empty

# Vet check
go vet ./...

# Module tidiness
go mod tidy  # Git status should be clean

# Test with race detector (all platforms)
go test -race ./...
```

## Test-Specific Scenarios

### Testing WebSub Protocol

```bash
# All WebSub tests
go test -race ./websub ./api

# Protocol compliance tests (38 tests)
go test ./websub -v

# Hub integration tests (14 tests)
go test ./api -run "^TestWebSub" -v

# URL encoding edge cases
go test ./websub -run "FeedURL" -v

# Hub error handling
go test ./websub -run "Error" -v

# Concurrent ping operations
go test ./websub -run "Concurrent" -v

# Feed lifecycle (new, update, delete, like, reply)
go test ./api -run "Ping" -v
```

### Testing Feed Generation

```bash
# All feed tests
go test ./feed -v

# RSS generation
go test ./feed -run "BuildFeed" -v

# Format negotiation (JSON vs XML)
go test ./api -run "FeedFormat" -v

# Link header presence (WebSub)
go test ./api -run "Header" -v
```

### Testing Data Layer

```bash
# All database operations
go test ./db -v

# User CRUD
go test ./db -run "User" -v

# Item CRUD
go test ./db -run "Item" -v

# Like operations
go test ./db -run "Like" -v

# Transaction safety
go test -race ./db
```

### Testing Authentication

```bash
# Auth-required endpoints
go test ./api -run "Auth" -v

# Blocklist enforcement
go test ./api -run "Blocklist" -v

# Email validation
go test ./api -run "Email" -v
```

## Troubleshooting

### Tests Hang

```bash
# Kill with timeout, see what's hanging
timeout 30s go test -v ./package_name

# Run with verbose output to see last test
go test -v ./package_name
```

### Database Locked Error

SQLite limitation during concurrent writes:
```bash
# Run serially (slower but avoids lock contention)
go test -parallel 1 ./...

# Or just the problematic test
go test -run "ConcurrentPings" ./api
```

### Import Cycle Errors

```bash
# Check for circular imports
go mod tidy
go mod verify
```

### Memory Issues

```bash
# Run tests one at a time
go test -p 1 ./...

# Limit parallel execution
go test -parallel 2 ./...
```

## Environment Variables

### Testing Configuration

```bash
# Enable verbose output
VERBOSE=1 go test ./...

# Disable cache
go test -count=1 ./...

# Run with timeout
TIMEOUT=60s go test ./...
```

### CI Environment

```bash
# Mimic CI environment
export CI=true
go build ./...
go test -race ./...
gofmt -l .
go vet ./...
go mod tidy
```

## Test Documentation

### WebSub Tests

Detailed documentation in [`websub/TESTING.md`](websub/TESTING.md):
- 38 protocol unit tests
- 14 API integration tests
- URL encoding, HTTP compliance, error handling
- Concurrency and load testing
- RFC 6685 compliance verification

### Running Full WebSub Suite

```bash
# Read testing guide
cat websub/TESTING.md

# Run all WebSub tests
go test -race ./websub ./api -run "WebSub|Pinger"

# View test coverage
go test -cover ./websub ./api
```

## Best Practices

1. **Before Commit**: Always run `go test -race ./...`
2. **Before Push**: Run full CI checks from "CI Pipeline Commands" above
3. **Coverage**: Aim for > 80% coverage on critical paths
4. **Naming**: Test names clearly describe what they test
5. **Isolation**: Tests don't depend on execution order
6. **Speed**: Most tests complete in < 100ms

## Summary

| Command | Purpose | Time |
|---------|---------|------|
| `go test ./...` | Run all tests | ~30s |
| `go test -race ./...` | All tests + race detector | ~60s |
| `go test -cover ./...` | Coverage for all packages | ~30s |
| `go test ./websub` | WebSub protocol tests | ~5s |
| `go test ./api -run WebSub` | WebSub integration tests | ~3s |
| `go test -run TestName -v` | Single test with details | <1s |

For more details on the WebSub test suite, see [`websub/TESTING.md`](websub/TESTING.md).
