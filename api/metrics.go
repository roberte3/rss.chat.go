package api

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metrics for the application
type Metrics struct {
	HTTPRequestsTotal   prometheus.Counter
	HTTPRequestDuration prometheus.Histogram
	HTTPErrorsTotal     prometheus.Counter
	DBQueriesTotal      prometheus.Counter
	DBQueryDuration     prometheus.Histogram
	ActiveConnections   prometheus.Gauge
	FeedsPublishedTotal prometheus.Counter
	WebSubPingsTotal    prometheus.Counter
}

// InitMetrics creates and registers all Prometheus metrics
func InitMetrics() *Metrics {
	return &Metrics{
		HTTPRequestsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		}),
		HTTPRequestDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds",
			Buckets: prometheus.DefBuckets,
		}),
		HTTPErrorsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "http_errors_total",
			Help: "Total number of HTTP errors",
		}),
		DBQueriesTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "db_queries_total",
			Help: "Total number of database queries",
		}),
		DBQueryDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "Database query latency in seconds",
			Buckets: prometheus.DefBuckets,
		}),
		ActiveConnections: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "active_connections",
			Help: "Number of active WebSocket connections",
		}),
		FeedsPublishedTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "feeds_published_total",
			Help: "Total number of feeds published",
		}),
		WebSubPingsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "websub_pings_total",
			Help: "Total number of WebSub hub pings",
		}),
	}
}

// Global metrics instance
var globalMetrics *Metrics

// GetMetrics returns the global metrics instance
func GetMetrics() *Metrics {
	if globalMetrics == nil {
		globalMetrics = InitMetrics()
	}
	return globalMetrics
}
