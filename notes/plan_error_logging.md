# Plan: Better Error Logging

## Overview
Implement structured, contextual error logging throughout the codebase to improve debugging, monitoring, and operational visibility.

**Priority**: High  
**Estimated Time**: 3-4 days  
**Blocking Issues**: None  
**Enables**: Debugging in production, error tracking, operational dashboards

---

## Current State

### What Works
- `fmt.Printf` for warnings in writes.go (e.g., mention/hashtag storage failures)
- HTTP error responses with `RespondError()` (returns JSON errors to clients)
- Database panic/error handling with `fmt.Errorf` wrapping

### What's Missing
- No structured logging (all output goes to stdout)
- No request tracing/request IDs
- No log levels (INFO, WARN, ERROR, DEBUG)
- No timestamp consistent formatting
- No integration with external log aggregation
- No correlation between API requests and internal errors
- No HTTP access logs

---

## Architecture

### Logging Framework
- Use `log/slog` (Go 1.21+) for structured logging
  - Built-in, no external dependencies
  - JSON output support for log aggregation
  - Request IDs via slog.With() context
  - Leveled output (DEBUG, INFO, WARN, ERROR)

### Log Levels & Usage
```
ERROR   - Failed operations that affect user (post creation failed, auth failed)
WARN    - Degraded operations (mention extraction failed, but post created)
INFO    - Key lifecycle events (user created, post published)
DEBUG   - Detailed execution flow (query executed, cache hit/miss)
```

### Implementation Strategy

**Phase 1: Core Infrastructure**
1. Initialize slog logger in main.go with:
   - Console handler (dev) or JSON handler (production)
   - Configurable level via config.json
   - Request ID generation (uuid)
2. Add request ID middleware to handler.go
   - Inject into context.Context for all requests
   - Pass to slog via slog.With()
3. Create package-level logger instances in each module

**Phase 2: Integration Points**
1. Database layer (db/)
   - Log slow queries (>100ms)
   - Log failed transactions
   - Log schema migrations
2. API layer (api/)
   - Log authentication failures (without exposing secrets)
   - Log validation errors (bad params, missing fields)
   - Log HTTP handler panics
3. Publishing layer (publish/)
   - Log feed generation start/completion
   - Log WebSub hub pings
4. Auth layer (email/)
   - Log email send attempts (not success/failure to avoid logging addresses)

**Phase 3: Error Response Enhancement**
1. Attach error ID to HTTP responses
   - Error responses include `"errorId": "req-12345"` for tracing
   - Client can report error ID for support
2. Structured error types
   - Custom error structs with code, message, details
   - Example: `ValidationError{Field: "email", Message: "invalid format"}`

---

## Implementation Details

### Code Changes by Module

**main.go**
- Initialize slog logger
- Configure output (json vs console)
- Set default level (info in prod, debug in dev)
- Pass logger to Handler

**api/handler.go**
- Add RequestLogger middleware
- Generate request ID (uuid v4)
- Log request start: method, path, IP
- Log request end: status code, latency, error (if any)
- Context injection for child handlers

**api/auth_endpoints.go**
- Log auth failures (invalid secret, user not found)
- Log password confirmation email sends (without email address)
- Avoid logging credentials in any form

**api/writes.go**
- Log post creation start/end
- Log mention/hashtag extraction warnings at WARN level
- Log media upload attempts at DEBUG level

**db/users.go, db/items.go**
- Log slow queries (implement timing)
- Log transaction rollbacks
- Log constraint violations (user exists, duplicate entry)

**publish/publisher.go**
- Log feed generation timings
- Log feed publish success/failure
- Log WebSub hub notifications

---

## Configuration

Add to config.json:
```json
{
  "logging": {
    "level": "info",           // debug, info, warn, error
    "format": "json",          // json or console
    "includeSource": false,    // Include file:line in output
    "requestIDHeader": "X-Request-ID"  // Header for incoming request IDs
  }
}
```

---

## Testing Strategy

**Unit Tests**
- Mock logger, verify correct log calls
- Test log level filtering
- Test request ID propagation through context

**Integration Tests**
- Full request cycle with logging enabled
- Verify error responses include error ID
- Test log output format (JSON)

**Manual Testing**
- Run server with debug logging
- Trigger various error conditions
- Verify log output is machine-readable

---

## Rollout Plan

1. **Week 1 Day 1**: Core infrastructure (slog, middleware)
2. **Week 1 Day 2-3**: Integration into critical paths (auth, writes, DB)
3. **Week 1 Day 4**: Error response enhancement, tests
4. **Week 2**: Deploy to staging, monitor log volume, adjust levels

---

## Success Criteria

- [ ] All ERROR level events logged with context
- [ ] Request IDs trackable through logs
- [ ] Log output parseable by log aggregation tools
- [ ] No sensitive data in logs (credentials, emails in auth failures)
- [ ] Log volume reasonable (not spammy at INFO level)
- [ ] Performance impact <5% (logging overhead)
- [ ] 100+ new log-related tests
- [ ] Integration tests verify error IDs returned to clients

---

## Future Enhancements

- Log sampling for high-volume events
- Metrics export (errors/sec, latency p50/p95/p99)
- Distributed tracing integration (OpenTelemetry)
- Structured context (user ID, post ID, feed name in logs)
