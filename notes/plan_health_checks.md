# Plan: Health Check Endpoints

## Overview
Implement comprehensive health check endpoints for monitoring server status, dependencies, and readiness for traffic. Enable load balancers and monitoring systems to detect and respond to failures.

**Priority**: High  
**Estimated Time**: 2-3 days  
**Blocking Issues**: None  
**Enables**: Load balancer integration, automated failover, monitoring dashboards

---

## Current State

### What Works
- `/health` endpoint (returns "OK" as plain text)
- Server starts successfully if databases can be opened

### What's Missing
- Detailed health status (which components are healthy?)
- Readiness checks (can accept traffic?)
- Liveness checks (is server responsive?)
- Dependency health (database, media DB, config)
- Response time / latency metrics
- Structured health response format

---

## Health Check Endpoints

### 1. `/health` (Liveness Probe)
**Purpose**: Is the server responsive? Used by load balancers to detect dead instances.

**Endpoint**: `GET /health`  
**Response**: 
```json
{
  "status": "ok",
  "timestamp": "2026-09-25T12:34:56Z",
  "uptime": 3600,
  "version": "1.2.0"
}
```
**Status Codes**:
- `200 OK` - Server is alive and responding
- `503 Service Unavailable` - Server shutting down or degraded

**Timing**: <50ms (no dependencies checked)

---

### 2. `/healthz` (Kubernetes-style health)
**Purpose**: Minimal liveness check for Kubernetes probes.

**Endpoint**: `GET /healthz`  
**Response**: Plain text `ok` (or empty body)  
**Status Codes**:
- `200 OK` - Alive
- `503` - Not ready

**Timing**: <10ms

---

### 3. `/ready` (Readiness Probe)
**Purpose**: Is the server ready to accept traffic? Checks all dependencies.

**Endpoint**: `GET /ready`  
**Response**:
```json
{
  "status": "ready",
  "timestamp": "2026-09-25T12:34:56Z",
  "dependencies": {
    "database": {"status": "ok", "latency_ms": 2},
    "media_database": {"status": "ok", "latency_ms": 3},
    "feeds_database": {"status": "ok", "latency_ms": 1},
    "config": {"status": "ok"}
  },
  "checks": [
    {"name": "can_read_items", "status": "ok"},
    {"name": "can_write_items", "status": "ok"}
  ]
}
```
**Status Codes**:
- `200 OK` - Ready for traffic
- `503 Service Unavailable` - Not ready (failed dependency)

**Timing**: <200ms (includes dependency checks)

---

### 4. `/health/detailed` (Observability endpoint)
**Purpose**: Full system health for dashboards and monitoring.

**Endpoint**: `GET /health/detailed`  
**Response**:
```json
{
  "status": "ok",
  "timestamp": "2026-09-25T12:34:56Z",
  "version": "1.2.0",
  "uptime_seconds": 3600,
  "goroutines": 42,
  "memory": {
    "alloc_mb": 125.5,
    "total_mb": 156.3,
    "sys_mb": 180.1
  },
  "database": {
    "status": "ok",
    "latency_ms": 2,
    "open_connections": 5,
    "max_connections": 25,
    "last_ping": "2026-09-25T12:34:55Z"
  },
  "media_database": {
    "status": "ok",
    "latency_ms": 3,
    "open_connections": 2,
    "last_ping": "2026-09-25T12:34:55Z"
  },
  "feeds_database": {
    "status": "ok",
    "latency_ms": 1
  },
  "config": {
    "status": "ok",
    "websub_enabled": false,
    "feeds_in_database": false
  },
  "blocklist": {
    "status": "ok",
    "last_sync": "2026-09-25T12:34:00Z",
    "entries": 150
  }
}
```
**Status Codes**: 200 OK (always returns details, check `status` field)

---

## Implementation Details

### Code Structure

**api/health.go** (new file)
```go
type HealthResponse struct {
    Status        string      `json:"status"`
    Timestamp     time.Time   `json:"timestamp"`
    Uptime        int64       `json:"uptime_seconds,omitempty"`
    Version       string      `json:"version,omitempty"`
    Dependencies  map[string]DependencyHealth `json:"dependencies,omitempty"`
}

type DependencyHealth struct {
    Status     string `json:"status"` // ok, degraded, error
    LatencyMS  int    `json:"latency_ms,omitempty"`
    Details    string `json:"details,omitempty"`
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) { ... }
func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) { ... }
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) { ... }
func (h *Handler) HealthDetailed(w http.ResponseWriter, r *http.Request) { ... }
```

**Dependencies to Check**

1. **Main Database** (main.db)
   - Query: `SELECT 1` with 50ms timeout
   - If fails or >100ms: degraded/unhealthy

2. **Media Database** (media.db)
   - Query: `SELECT 1` with 50ms timeout
   - If fails: degraded (not critical)

3. **Feeds Database** (feeds.db, if enabled)
   - Query: `SELECT 1` with 50ms timeout
   - If fails: degraded (not critical)

4. **Config** (config.json exists and valid)
   - Check file exists and readable
   - Check required fields present

5. **Blocklist** (blocklist.json sync)
   - Check last sync time <1 hour
   - If stale: degraded warning

6. **Write Capability** (can create/update items)
   - Optional: Try INSERT/UPDATE on test row
   - Only in detailed checks (adds latency)

### Handler Registration

```go
register("GET", "/health", h.Health)
register("GET", "/healthz", h.Healthz)
register("GET", "/ready", h.Ready)
register("GET", "/health/detailed", h.HealthDetailed)
```

All endpoints available at both `/path` and `/api/path`.

---

## Status Determination

### Overall Status Calculation
```
if main_db.status == error:
    status = "unavailable"
elif any dependency (except optional) is degraded:
    status = "degraded"
elif all dependencies ok:
    status = "ok"
```

### Readiness Status
- `/ready` returns 200 only if `status == "ok"`
- Returns 503 if status is "degraded" or "unavailable"
- Load balancer removes server from pool on 503

---

## Testing Strategy

**Unit Tests**
- Mock database failures, verify status responses
- Test timeout handling (database slow)
- Test dependency combinations

**Integration Tests**
- Full server startup, verify health endpoints respond
- Close databases, verify degraded status
- Verify status propagates correctly

**Load Testing**
- Health checks under load (concurrent requests)
- Verify health checks don't block other requests
- Measure health endpoint latency

---

## Configuration

Add to config.json:
```json
{
  "health": {
    "enabled": true,
    "db_timeout_ms": 50,
    "include_memory_stats": true,
    "include_goroutines": true,
    "check_blocklist_age": true
  }
}
```

---

## Monitoring Integration

### Kubernetes Probes
```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 5
```

### Load Balancer Health Checks
```
HTTP GET /ready
Expected: 200 OK
Timeout: 5 seconds
Unhealthy threshold: 2 failures
```

### Monitoring Dashboard
- Poll `/health/detailed` every 60 seconds
- Chart: latencies, dependency status changes
- Alert: Status transitions to "degraded" or "unavailable"

---

## Rollout Plan

1. **Day 1**: Core health endpoints (`/health`, `/ready`)
2. **Day 2**: Detailed endpoint, dependency checks
3. **Day 3**: Tests, load testing, documentation

---

## Success Criteria

- [ ] `/health` responds in <50ms
- [ ] `/ready` checks all dependencies in <200ms
- [ ] Dependency failures detected within 5 seconds
- [ ] Health endpoints work when main server is under load
- [ ] Kubernetes integration tested
- [ ] 40+ health check tests
- [ ] No performance impact on main request path
- [ ] Documentation for ops teams

---

## Future Enhancements

- HTTP status codes as Prometheus metrics
- Trace IDs in health responses for debugging
- Custom health checks (client-provided)
- Health check aggregation (multiple servers reporting to cluster health)
