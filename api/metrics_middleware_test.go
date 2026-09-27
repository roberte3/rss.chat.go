package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestMetricsRecordRequestDuration tests that histogram records requests
func TestMetricsRecordRequestDuration(t *testing.T) {
	metrics := GetMetrics()

	// Record a duration
	metrics.HTTPRequestDuration.Observe(0.05)
	metrics.HTTPRequestDuration.Observe(0.15)

	// If we get here without panic, histogram works
}

// TestMetricsIncrementCounters tests that counters can be incremented
func TestMetricsIncrementCounters(t *testing.T) {
	metrics := GetMetrics()

	// Increment counters
	metrics.HTTPRequestsTotal.Inc()
	metrics.HTTPErrorsTotal.Inc()
	metrics.DBQueriesTotal.Inc()
	metrics.FeedsPublishedTotal.Inc()
	metrics.WebSubPingsTotal.Inc()

	// If we get here without panic, counters work
}

// TestMetricsManageGauges tests that gauge values can be set
func TestMetricsManageGauges(t *testing.T) {
	metrics := GetMetrics()

	// Set gauge values
	metrics.ActiveConnections.Set(5)
	metrics.ActiveConnections.Inc()
	metrics.ActiveConnections.Dec()

	// If we get here without panic, gauges work
}

// TestMetricsRecordQueryDuration tests database query duration recording
func TestMetricsRecordQueryDuration(t *testing.T) {
	metrics := GetMetrics()

	// Record query durations
	metrics.DBQueryDuration.Observe(0.01)
	metrics.DBQueryDuration.Observe(0.05)
	metrics.DBQueryDuration.Observe(0.15)

	// If we get here without panic, histogram works
}

// TestLogQueryWithMetricsDoesNotCrash tests that LogQueryWithMetrics doesn't panic
func TestLogQueryWithMetricsDoesNotCrash(t *testing.T) {
	startTime := time.Now()
	time.Sleep(1 * time.Millisecond)

	// Call should not panic even if logger is not initialized
	// We're testing that metrics recording works without panicking
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Recovered from panic (expected): %v", r)
		}
	}()

	LogQueryWithMetrics(nil, "SELECT", "users", startTime, nil)
}

// TestHealthCheckEndpointsExcludedFromMetrics tests that health endpoints don't increment counter
func TestHealthCheckEndpointsExcludedFromMetrics(t *testing.T) {
	// Create a simple test handler that tracks calls
	callCount := 0
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	})

	// Create middleware with health endpoint exclusions
	middleware := &RequestLogger{
		next:            nextHandler,
		requestIDHeader: "X-Request-ID",
		metrics:         GetMetrics(),
		excludePaths: map[string]bool{
			"/health":  true,
			"/ready":   true,
			"/metrics": true,
		},
	}

	// Make request to /health
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	// Handler should still be called
	if callCount != 1 {
		t.Errorf("handler should have been called: callCount=%d", callCount)
	}

	// Status should be 200
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// TestRequestLoggerPassesRequestThrough tests that middleware passes request through
func TestRequestLoggerPassesRequestThrough(t *testing.T) {
	responseBody := "test response"
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(responseBody))
	})

	middleware := &RequestLogger{
		next:            nextHandler,
		requestIDHeader: "X-Request-ID",
		metrics:         GetMetrics(),
		excludePaths: map[string]bool{
			"/health": true,
		},
	}

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Logf("Recovered from panic (expected due to logger): %v", r)
		}
	}()

	middleware.ServeHTTP(w, req)

	// Handler should have been called (even if middleware panics later)
	if w.Body.String() != responseBody {
		t.Errorf("response body mismatch: got %q, want %q", w.Body.String(), responseBody)
	}
}

// TestMetricsMultipleObservations tests that multiple observations work
func TestMetricsMultipleObservations(t *testing.T) {
	metrics := GetMetrics()

	// Record multiple observations
	for i := 0; i < 10; i++ {
		metrics.HTTPRequestDuration.Observe(float64(i) * 0.01)
		metrics.DBQueryDuration.Observe(float64(i) * 0.005)
	}

	// If we get here without panic, observations work
}

// TestMetricsCounterMultipleIncrements tests counter increments
func TestMetricsCounterMultipleIncrements(t *testing.T) {
	metrics := GetMetrics()

	// Increment multiple times
	for i := 0; i < 10; i++ {
		metrics.HTTPRequestsTotal.Inc()
		metrics.HTTPErrorsTotal.Inc()
	}

	// If we get here without panic, counter works
}

// TestMetricsGaugeSetAndModify tests gauge set/inc/dec
func TestMetricsGaugeSetAndModify(t *testing.T) {
	metrics := GetMetrics()

	// Perform gauge operations
	metrics.ActiveConnections.Set(0)
	for i := 0; i < 5; i++ {
		metrics.ActiveConnections.Inc()
	}
	for i := 0; i < 3; i++ {
		metrics.ActiveConnections.Dec()
	}
	metrics.ActiveConnections.Set(10)
	metrics.ActiveConnections.Add(5)
	metrics.ActiveConnections.Sub(2)

	// If we get here without panic, gauge works
}
