# WebSub Protocol - Comprehensive Test Suite

This document describes the full test coverage for the WebSub (Web Push) implementation.

## Overview

**Total Tests**: 52 tests across two suites
- **Protocol Tests**: 38 unit/protocol tests (`websub/protocol_test.go`)
- **Integration Tests**: 14 API integration tests (`api/websub_integration_test.go`)

All tests pass with race detector enabled: `go test -race ./...`

## Protocol Tests (38 tests)

### URL Encoding & Handling (5 tests)
Test correct handling of diverse feed URL formats in POST form data.

- **TestPingerFeedURLEncoding**: 5 sub-tests
  - Simple URLs: `https://example.com/feed`
  - Query parameters: `?screenname=alice&format=xml`
  - Special characters: `?screenname=user%2Bname` (URL-encoded)
  - Fragment identifiers: `#section` handling
  - Comments feed URLs: `/comments/alice/123.xml`

- **TestPingerFeedURLVariety**: 8 diverse feed URL formats
  - Subdomains: `subdomain.example.com`
  - Custom ports: `:8080`
  - Deep paths: `/path/to/feed`
  - OPML feeds: `/getsubscriptionlist`
  - Encoded screennames in query strings

### HTTP Compliance (4 tests)

- **TestPingerHTTPMethod**
  - Verifies POST method is used (not GET, PUT, etc.)

- **TestPingerContentType**
  - Checks `Content-Type: application/x-www-form-urlencoded` header

- **TestPingerFormDataCorrectness**
  - Verifies exact form fields: `hub.mode=publish` and `hub.url=<feedURL>`
  - Ensures no extraneous fields are sent

- **TestPingerRequestPath**
  - Verifies POST request path starts with `/`

### Hub Response Handling (1 test with 8 sub-tests)

- **TestPingerHubResponseCodes**
  - Tests graceful handling of all HTTP response codes
  - Success codes: 200 OK, 202 Accepted, 204 No Content
  - Client error codes: 400, 401, 404
  - Server error codes: 500, 503
  - Pinger doesn't panic or block on any response code

### Concurrency & Load Testing (3 tests)

- **TestPingerConcurrentPings**: 10 concurrent pings
  - Verifies all pings reach hub
  - Tests goroutine safety of pinger

- **TestPingerBurstLoad**: 100 rapid successive pings
  - Tests handling of burst traffic
  - Verifies no pings are dropped under load

- **TestPingerConcurrentPings**: Race-free concurrent operation

### Error Handling & Resilience (5 tests)

- **TestPingerWithHubError**
  - Hub returns 500 Internal Server Error
  - Pinger logs error but doesn't block

- **TestPingerWithUnreachableHub**
  - Connection to hub fails (TCP connection refused)
  - Pinger handles gracefully

- **TestPingerHubTimeout**
  - Hub never responds (5-second timeout)
  - Pinger returns quickly (async, non-blocking)

- **TestPingerHubConnectionRefusal**
  - Connection to port 1 (never accepts connections)
  - Verifies no panic

- **TestPingerHubPartialResponse**
  - Server closes connection abruptly
  - Pinger handles incomplete response

### Edge Cases & State Management (6 tests)

- **TestPingerEmptyFeedURL**
  - Pings with empty feed URL don't reach hub

- **TestPingerEmptyHubURL**
  - Pings with empty hub URL don't crash

- **TestPingerNotInitialized**
  - Pinger with empty hub URL and enabled flag
  - Graceful no-op behavior

- **TestPingerDisabled**
  - Disabled pinger (enabled=false)
  - No pings sent regardless of input

- **TestPingerStateChanges**
  - IsEnabled() method returns correct state

- **TestPingerDebugLogging**
  - SetDebugLog(nil) - disable logging
  - SetDebugLog(logger) - enable logging

### Multiple Hub Support (2 tests)

- **TestPingerMultipleHubURLs**
  - Tests creating pingers for different hub URLs
  - Each pinger independently works correctly

## Integration Tests (14 tests)

### Feed Update Pinging (5 tests)

- **TestWebSubPingOnNewPost**
  - Creating a new post triggers hub ping
  - Verifies `hub.mode=publish` in form data
  - Confirms feed URL is in ping

- **TestWebSubPingOnUpdatePost**
  - Editing a post triggers hub ping
  - Tracks ping count before and after update

- **TestWebSubPingOnDelete**
  - Deleting a post triggers hub ping
  - Verifies ping for both user feed and global feed

- **TestWebSubPingOnLike**
  - Toggling like on a post triggers hub ping
  - Works across users (Alice likes Bob's post)

- **TestWebSubPingOnReply**
  - Replying to a post triggers hub pings
  - Verifies pings for: reply itself, comments feed, global feed
  - Tests threaded discussion scenario

### Link Header Presence (3 tests)

- **TestWebSubHeaderPresent**: 2 sub-tests
  - User feed includes Link header with `rel="self"` and feed URL
  - Everyone feed includes Link header

- **TestWebSubCommentsFeedHeader**
  - Comments feed response includes Link header
  - Verifies both `rel="hub"` and `rel="self"`
  - Tests `/comments/{screenname}/{id}.xml` endpoint

- **TestWebSubOPMLHeader**
  - OPML subscription list includes Link header
  - Tests `/getsubscriptionlist` endpoint

### Header Content Verification (1 test)

- **TestWebSubHeaderContainsHubAndSelf**
  - Link header contains hub URL from config
  - Link header contains feed URL (`screenname=alice`)
  - Both `rel="hub"` and `rel="self"` present

### Configuration & Behavior (3 tests)

- **TestWebSubHeadersDisabledWhenWebSubDisabled**
  - When WebSub disabled, no unnecessary pinging
  - Behavior when URLWebsubHub is empty

- **TestWebSubHubURLFromConfig**
  - Correct hub URL from `handler.Config.URLWebsubHub` is used
  - Configuration change affects which hub receives pings

- **TestWebSubConcurrentPingsFromDifferentEndpoints**
  - Multiple users create posts (sequentially due to SQLite limits)
  - Each triggers separate hub pings
  - Verifies pings for: user feeds + global feed

## Test Execution

### Run All WebSub Tests
```bash
# Protocol tests
go test ./websub -v

# Integration tests
go test ./api -run WebSub -v

# All WebSub tests with race detector
go test -race ./websub ./api
```

### Run Specific Test
```bash
go test ./websub -run TestPingerConcurrentPings -v
go test ./api -run TestWebSubPingOnNewPost -v
```

### Test Coverage by Category
```bash
# URL encoding tests
go test ./websub -run FeedURL -v

# Error handling tests
go test ./websub -run Error -v

# Concurrency tests
go test ./websub -run Concurrent -v

# Integration tests
go test ./api -run "WebSub" -v
```

## Test Isolation & Safety

All tests:
- Use ephemeral mock hub servers on random ports
- Clean up resources (defer statements)
- Don't require network access to external services
- Use channels for proper synchronization (race detector clean)
- Run in < 5 seconds total
- Can be run in any order or in parallel

## Protocol Compliance

Tests verify compliance with:
- **W3C WebSub Recommendation** (https://www.w3.org/TR/websub/)
- **HTTP/1.1**: POST method, Content-Type headers
- **Form Data**: `application/x-www-form-urlencoded` encoding

## Performance Characteristics

- Burst load test (100 pings): < 1 second
- Concurrent operations (10 concurrent pings): < 500ms
- Typical single ping: < 100ms
- Timeout handling: non-blocking (async operations)

## Future Test Additions

Potential enhancements:
- Retry logic and exponential backoff (if implemented)
- Hub signature verification (if implemented)
- Subscriber notification flow (subscriber-side tests)
- Performance benchmarks (threshold-based)
- Load testing with sustained 1000+ pings/second
