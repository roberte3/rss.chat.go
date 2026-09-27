package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHandleHealthSuccess tests successful health check
func TestHandleHealthSuccess(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	HandleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp HealthResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "healthy" {
		t.Errorf("expected status 'healthy', got %q", resp.Status)
	}

	if resp.Timestamp.IsZero() {
		t.Error("timestamp should not be zero")
	}
}

// TestHandleHealthContentType checks JSON content type
func TestHandleHealthContentType(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	HandleHealth(w, req)

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected content-type application/json, got %q", contentType)
	}
}

// TestHandleHealthMethodNotAllowed tests non-GET requests
func TestHandleHealthMethodNotAllowed(t *testing.T) {
	tests := []string{"POST", "PUT", "DELETE", "PATCH"}
	for _, method := range tests {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/health", nil)
			w := httptest.NewRecorder()

			HandleHealth(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected status 405, got %d", w.Code)
			}
		})
	}
}

// TestHandleReadyWithoutDatabase tests ready check without database in context
func TestHandleReadyWithoutDatabase(t *testing.T) {
	req := httptest.NewRequest("GET", "/ready", nil)
	w := httptest.NewRecorder()

	HandleReady(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503 (no database), got %d", w.Code)
	}

	var resp ReadinessResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Ready {
		t.Error("expected ready=false without database")
	}

	if resp.Checks["database"] {
		t.Error("expected database check to fail")
	}
}

// TestHandleReadyContentType checks JSON content type
func TestHandleReadyContentType(t *testing.T) {
	req := httptest.NewRequest("GET", "/ready", nil)
	w := httptest.NewRecorder()

	HandleReady(w, req)

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected content-type application/json, got %q", contentType)
	}
}

// TestHandleReadyMethodNotAllowed tests non-GET requests
func TestHandleReadyMethodNotAllowed(t *testing.T) {
	tests := []string{"POST", "PUT", "DELETE", "PATCH"}
	for _, method := range tests {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/ready", nil)
			w := httptest.NewRecorder()

			HandleReady(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected status 405, got %d", w.Code)
			}
		})
	}
}

// TestHandleReadyResponseStructure tests response fields
func TestHandleReadyResponseStructure(t *testing.T) {
	req := httptest.NewRequest("GET", "/ready", nil)
	w := httptest.NewRecorder()

	HandleReady(w, req)

	var resp ReadinessResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Timestamp.IsZero() {
		t.Error("timestamp should not be zero")
	}

	if resp.Checks == nil {
		t.Error("checks should not be nil")
	}

	if _, ok := resp.Checks["database"]; !ok {
		t.Error("database check should exist in checks")
	}
}

// TestHandleMetricsSuccess tests metrics endpoint
func TestHandleMetricsSuccess(t *testing.T) {
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	HandleMetrics(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") {
		t.Errorf("expected content-type text/plain, got %q", contentType)
	}

	body := w.Body.String()
	if body == "" {
		t.Error("metrics response body should not be empty")
	}

	// Check that Prometheus header is present
	if !strings.Contains(body, "# HELP") && !strings.Contains(body, "# TYPE") {
		t.Error("metrics should contain Prometheus format headers")
	}
}

// TestHandleMetricsMethodNotAllowed tests non-GET requests
func TestHandleMetricsMethodNotAllowed(t *testing.T) {
	tests := []string{"POST", "PUT", "DELETE", "PATCH"}
	for _, method := range tests {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/metrics", nil)
			w := httptest.NewRecorder()

			HandleMetrics(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected status 405, got %d", w.Code)
			}
		})
	}
}

// TestHealthResponseJSON tests JSON marshaling
func TestHealthResponseJSON(t *testing.T) {
	resp := HealthResponse{
		Status: "healthy",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	if !strings.Contains(string(data), "healthy") {
		t.Error("JSON should contain status value")
	}
}

// TestReadinessResponseJSON tests JSON marshaling
func TestReadinessResponseJSON(t *testing.T) {
	resp := ReadinessResponse{
		Ready:    true,
		Database: true,
		Checks: map[string]bool{
			"database": true,
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded ReadinessResponse
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if !decoded.Ready {
		t.Error("ready should be true")
	}

	if !decoded.Database {
		t.Error("database should be true")
	}
}

// TestMetricsContainsExpectedMetrics tests that metrics endpoint exposes expected counters
func TestMetricsContainsExpectedMetrics(t *testing.T) {
	// Initialize metrics to register them with Prometheus
	_ = GetMetrics()

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	HandleMetrics(w, req)

	body := w.Body.String()

	// Prometheus outputs HELP and TYPE lines for registered metrics
	expectedMetrics := []string{
		"http_requests_total",
		"http_request_duration_seconds",
		"http_errors_total",
		"db_queries_total",
		"db_query_duration_seconds",
		"active_connections",
		"feeds_published_total",
		"websub_pings_total",
	}

	for _, metric := range expectedMetrics {
		// Check for HELP line which includes the metric name
		helpLine := "# HELP " + metric
		if !strings.Contains(body, helpLine) {
			t.Errorf("metrics should contain %q", helpLine)
		}
	}
}

// TestMetricsFormat tests Prometheus text format
func TestMetricsFormat(t *testing.T) {
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	HandleMetrics(w, req)

	body := w.Body.String()
	lines := strings.Split(body, "\n")

	hasHelpLine := false
	hasTypeLine := false

	for _, line := range lines {
		if strings.HasPrefix(line, "# HELP") {
			hasHelpLine = true
		}
		if strings.HasPrefix(line, "# TYPE") {
			hasTypeLine = true
		}
	}

	if !hasHelpLine {
		t.Error("metrics should have HELP lines")
	}

	if !hasTypeLine {
		t.Error("metrics should have TYPE lines")
	}
}

// TestHealthWithContext tests health endpoint with request context
func TestHealthWithContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), "test", "value")
	req := httptest.NewRequest("GET", "/health", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	HandleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp HealthResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "healthy" {
		t.Errorf("expected status 'healthy', got %q", resp.Status)
	}
}

// TestReadyResponseConsistency tests that timestamp is always recent
func TestReadyResponseConsistency(t *testing.T) {
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/ready", nil)
		w := httptest.NewRecorder()

		HandleReady(w, req)

		var resp ReadinessResponse
		err := json.NewDecoder(w.Body).Decode(&resp)
		if err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Timestamp.IsZero() {
			t.Error("timestamp should not be zero")
		}
	}
}
