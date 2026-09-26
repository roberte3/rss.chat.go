# Plan: Metrics & Observability

## Overview
Implement comprehensive metrics collection and observability across the entire system. Enable production monitoring, performance analysis, and debugging through metrics export (Prometheus format) and correlation of logs, traces, and metrics.

**Priority**: High  
**Estimated Time**: 5-7 days  
**Blocking Issues**: Depends on error logging plan  
**Enables**: Performance dashboards, SLO tracking, anomaly detection, troubleshooting

---

## Current State

### What Works
- Error logging (when completed)
- HTTP server running with basic access patterns
- Database operations working

### What's Missing
- Request latency metrics (histogram, percentiles)
- Error rate tracking
- Business metrics (posts/hour, users/day, feed generation time)
- Resource usage (memory, goroutines, database connections)
- Query performance metrics
- Cache hit/miss rates
- WebSocket connection metrics
- Feature usage (mentions extracted, hashtags created, avatars uploaded)

---

## Metrics Architecture

### Three Pillars

1. **Logs** — Events with context and trace IDs
   - Implemented by error logging plan
   - Searchable, correlatable via request ID

2. **Metrics** — Quantitative measurements over time
   - Counter: Total requests, errors
   - Gauge: Active connections, memory usage
   - Histogram: Request latency, query duration
   - Export: Prometheus format `/metrics`

3. **Traces** — Detailed request flow through system
   - Currently: Request ID in logs
   - Future: Distributed tracing (OpenTelemetry)

---

## Metrics to Collect

### Request/HTTP Metrics
```
http_requests_total{method, path, status}           Counter
http_request_duration_seconds{method, path}         Histogram (p50, p95, p99)
http_errors_total{method, path, error_code}         Counter
http_active_connections                              Gauge
http_response_size_bytes{method, path}              Histogram
```

### Authentication Metrics
```
auth_attempts_total{result}                          Counter (success, failure)
auth_failures_total{reason}                          Counter (invalid_secret, user_not_found)
user_creation_total                                  Counter
blocklist_checks_total                               Counter
```

### Post/Content Metrics
```
posts_created_total{author}                          Counter
posts_updated_total                                  Counter
posts_deleted_total                                  Counter
posts_per_second                                     Gauge
replies_total                                        Counter
mentions_extracted_total                             Counter
hashtags_extracted_total                             Counter
```

### Avatar Metrics
```
avatars_uploaded_total                               Counter
avatar_size_bytes{percentile}                        Histogram
avatar_storage_total_bytes                           Gauge
```

### Feed/Publishing Metrics
```
feed_generation_duration_seconds{feed_type}          Histogram
feed_items_count{feed_type, user}                    Gauge
feed_size_bytes{feed_type}                           Histogram
feed_publish_total{storage_mode}                     Counter
websub_pings_total{status}                           Counter
```

### Database Metrics
```
db_query_duration_seconds{operation, table}          Histogram
db_queries_total{operation, table, status}           Counter
db_connection_pool_size                              Gauge
db_open_connections                                  Gauge
db_transaction_duration_seconds                      Histogram
db_locks_total{table}                                Counter
```

### WebSocket Metrics
```
websocket_connections_active                        Gauge
websocket_connections_total                         Counter
websocket_messages_sent_total{event_type}           Counter
websocket_messages_received_total                   Counter
websocket_disconnect_total{reason}                  Counter
```

### System Metrics
```
process_uptime_seconds                              Gauge
process_goroutines                                  Gauge
process_memory_bytes{type}                          Gauge (heap, stack, etc)
process_cpu_seconds_total                           Counter
```

### Custom Business Metrics
```
active_users                                        Gauge
posts_this_hour                                     Counter
feed_subscribers_total                              Gauge
average_post_length_bytes                           Gauge
```

---

## Implementation Strategy

### Phase 1: Prometheus Client Library
- Use `prometheus/client_golang` (battle-tested, Go stdlib metrics)
- No external dependencies beyond Prometheus client
- Exposes `/metrics` endpoint (standard)

### Phase 2: Core Instrumentation
1. HTTP middleware for request metrics
   - Latency histogram (labels: method, path, status)
   - Request counter
   - Error counter
2. Database operation metrics
   - Query duration histograms
   - Operation counters
3. Business logic metrics
   - Post creation counter
   - Mention extraction counter
   - Avatar upload counter

### Phase 3: Advanced Metrics
1. WebSocket metrics
2. Feature-specific counters
3. System resource metrics
4. Custom business KPIs

### Phase 4: Observability Dashboard
1. Prometheus data source
2. Grafana dashboards
3. Alert rules
4. Query examples

---

## Code Implementation

### metrics.go (new file)
```go
package api

import (
    "github.com/prometheus/client_golang/prometheus"
)

var (
    // HTTP metrics
    httpRequestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "http_request_duration_seconds",
            Buckets: []float64{.001, .01, .1, 1, 10},
        },
        []string{"method", "path", "status"},
    )
    
    httpRequestTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{Name: "http_requests_total"},
        []string{"method", "path", "status"},
    )
    
    // Post metrics
    postsCreatedTotal = prometheus.NewCounter(
        prometheus.CounterOpts{Name: "posts_created_total"},
    )
    
    mentionsExtractedTotal = prometheus.NewCounter(
        prometheus.CounterOpts{Name: "mentions_extracted_total"},
    )
    
    // Database metrics
    dbQueryDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "db_query_duration_seconds",
            Buckets: []float64{.001, .01, .05, .1, .5, 1},
        },
        []string{"operation", "table"},
    )
    
    // ... more metrics
)

func init() {
    prometheus.MustRegister(
        httpRequestDuration,
        httpRequestTotal,
        postsCreatedTotal,
        mentionsExtractedTotal,
        dbQueryDuration,
        // ... all metrics
    )
}
```

### HTTP Middleware Integration
```go
// In handler.go RegisterRoutes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
    // ... existing routes ...
    
    // Metrics endpoint
    mux.HandleFunc("GET /metrics", prometheusHandler)
    
    // Wrap mux with metrics middleware
    wrappedMux := metricsMiddleware(mux)
}

func metricsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        
        // Wrap response writer to capture status
        wrapped := &responseWriter{ResponseWriter: w}
        
        next.ServeHTTP(wrapped, r)
        
        // Record metrics
        duration := time.Since(start).Seconds()
        httpRequestDuration.WithLabelValues(
            r.Method, r.URL.Path, fmt.Sprint(wrapped.statusCode),
        ).Observe(duration)
        httpRequestTotal.WithLabelValues(
            r.Method, r.URL.Path, fmt.Sprint(wrapped.statusCode),
        ).Inc()
    })
}
```

### Database Instrumentation
```go
// In db/items.go
func GetRecentItems(conn *sql.DB, viewerScreenname string, maxCt int, baseURL string) ([]Item, error) {
    start := time.Now()
    defer func() {
        duration := time.Since(start).Seconds()
        dbQueryDuration.WithLabelValues("select", "items").Observe(duration)
    }()
    
    // ... existing query code ...
}
```

### Business Logic Instrumentation
```go
// In api/writes.go HandleNewPost
func (h *Handler) HandleNewPost(w http.ResponseWriter, r *http.Request, user *db.User) {
    // ... existing code ...
    
    itemID, err := db.AddItem(h.DB, newItem)
    if err != nil {
        RespondError(w, "Can't create post because "+err.Error())
        return
    }
    
    postsCreatedTotal.Inc()
    
    // Extract and store mentions
    if mentions, err := ExtractMentions(h.DB, sanitized); err == nil {
        mentionsExtractedTotal.Add(float64(len(mentions)))
        // ...
    }
    
    // ... rest of handler ...
}
```

---

## Prometheus Configuration

### prometheus.yml
```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'rss-chat'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics'
```

### Alert Rules (alerts.yml)
```yaml
groups:
  - name: rss-chat
    rules:
      - alert: HighErrorRate
        expr: rate(http_errors_total[5m]) > 0.05
        for: 5m
        annotations:
          summary: "High error rate (>5%)"
      
      - alert: DatabaseLatency
        expr: histogram_quantile(0.95, db_query_duration_seconds) > 1
        for: 5m
        annotations:
          summary: "Database queries p95 latency >1s"
      
      - alert: HighMemoryUsage
        expr: process_resident_memory_bytes > 500_000_000
        for: 5m
        annotations:
          summary: "Process memory >500MB"
```

---

## Grafana Dashboards

### Main Dashboard
- Request rate (requests/sec)
- Error rate (errors/sec)
- Latency (p50, p95, p99)
- Active connections
- Memory usage
- Goroutines
- Database connection pool

### Business Dashboard
- Posts created/hour
- Users created/day
- Mentions extracted
- Hashtags created
- Avatars uploaded
- Feed generation time

### Database Dashboard
- Query latency by operation
- Query count by table
- Connection pool status
- Transaction duration
- Lock contentions

### WebSocket Dashboard
- Active connections
- Message rate (sent/received)
- Connection duration
- Disconnect reasons

---

## Testing Strategy

**Unit Tests**
- Verify metrics increment correctly
- Test histogram bucket behavior
- Test label combinations

**Integration Tests**
- Full request cycle generates metrics
- Multiple requests show aggregation
- Metrics endpoint returns valid Prometheus format

**Load Tests**
- Metrics collection under 1000 req/sec
- Verify no performance regression
- Memory usage of metrics collection

---

## Rollout Plan

**Week 1 Day 1-2**: Core infrastructure (Prometheus client, middleware)  
**Week 1 Day 3**: Database & business logic instrumentation  
**Week 1 Day 4**: WebSocket and system metrics  
**Week 2 Day 1**: Grafana dashboards, alert rules  
**Week 2 Day 2-3**: Integration tests, load testing  

---

## Success Criteria

- [ ] `/metrics` endpoint returns valid Prometheus format
- [ ] Request latency tracked with p50/p95/p99
- [ ] All error paths record metrics
- [ ] Database operations instrumented (>90% of queries)
- [ ] Business metrics reflect actual user actions
- [ ] Grafana dashboards show meaningful trends
- [ ] No performance degradation (<2% overhead)
- [ ] 50+ metrics recorded
- [ ] 30+ integration tests
- [ ] Documentation for metrics interpretation

---

## Monitoring Strategy

### SLO Definition
- Request latency p99 < 500ms
- Error rate < 1%
- Uptime > 99.5%

### Alerting
- Error rate spike detected within 1 minute
- Database latency degradation
- Memory/goroutine leaks
- WebSocket connection drops

### Debugging Workflow
1. Alert fires (e.g., error rate spike)
2. Check Grafana for error distribution
3. Use trace IDs in logs to find root cause
4. Cross-reference with metrics for impact
5. Deploy fix and monitor recovery

---

## Integration with ELK Stack (Future)

If adopting log aggregation:
1. Logs (Elasticsearch) — Detailed events
2. Metrics (Prometheus) — Trends and aggregates
3. Traces (Jaeger) — Request flow
4. Correlation via request ID across all three

---

## Future Enhancements

- OpenTelemetry integration for distributed tracing
- Custom metrics for client libraries
- Metrics query language (PromQL) cheat sheet
- ML-based anomaly detection
- Performance regression detection in CI
- Automated capacity planning based on trends
