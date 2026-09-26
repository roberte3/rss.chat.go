package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/roberte3/rss.chat.go/log"
)

func TestRespondErrorWithIDStructured(t *testing.T) {
	// Initialize logger for testing
	log.Init(log.Config{
		Level:           "info",
		Format:          "console",
		IncludeSource:   false,
		RequestIDHeader: "X-Request-ID",
	})

	// Create a test request with request ID middleware
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "test-req-12345")

	// Add request ID to context (simulating what the middleware does)
	ctx := log.ContextWithRequestID(req.Context(), "test-req-12345")
	req = req.WithContext(ctx)

	// Create response writer
	w := httptest.NewRecorder()

	// Call the error response function
	RespondErrorWithID(w, req, "Test error message")

	// Verify response
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
	}

	// Parse response body
	var errorResp ErrorResponse
	err := json.NewDecoder(w.Body).Decode(&errorResp)
	if err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}

	// Verify error response structure
	if errorResp.Error != "Test error message" {
		t.Errorf("Expected error message 'Test error message', got '%s'", errorResp.Error)
	}

	if errorResp.ErrorID != "err-test-req-12345" {
		t.Errorf("Expected error ID 'err-test-req-12345', got '%s'", errorResp.ErrorID)
	}

	if errorResp.Code != "ERROR" {
		t.Errorf("Expected code 'ERROR', got '%s'", errorResp.Code)
	}

	if errorResp.Time == "" {
		t.Errorf("Expected non-empty time field")
	}

	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", w.Header().Get("Content-Type"))
	}
}

func TestRespondValidationError(t *testing.T) {
	log.Init(log.Config{
		Level:           "info",
		Format:          "console",
		IncludeSource:   false,
		RequestIDHeader: "X-Request-ID",
	})

	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-Request-ID", "test-req-67890")
	ctx := log.ContextWithRequestID(req.Context(), "test-req-67890")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	RespondValidationError(w, req, "email", "invalid format")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var errorResp ErrorResponse
	json.NewDecoder(w.Body).Decode(&errorResp)

	if errorResp.Code != "VALIDATION_ERROR" {
		t.Errorf("Expected code 'VALIDATION_ERROR', got '%s'", errorResp.Code)
	}

	if errorResp.ErrorID != "err-test-req-67890" {
		t.Errorf("Expected error ID 'err-test-req-67890', got '%s'", errorResp.ErrorID)
	}
}

func TestRespondAuthError(t *testing.T) {
	log.Init(log.Config{
		Level:           "info",
		Format:          "console",
		IncludeSource:   false,
		RequestIDHeader: "X-Request-ID",
	})

	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-Request-ID", "test-req-auth")
	ctx := log.ContextWithRequestID(req.Context(), "test-req-auth")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	RespondAuthError(w, req, "Invalid credentials")

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	var errorResp ErrorResponse
	json.NewDecoder(w.Body).Decode(&errorResp)

	if errorResp.Code != "AUTH_ERROR" {
		t.Errorf("Expected code 'AUTH_ERROR', got '%s'", errorResp.Code)
	}

	if errorResp.ErrorID != "err-test-req-auth" {
		t.Errorf("Expected error ID 'err-test-req-auth', got '%s'", errorResp.ErrorID)
	}
}

func TestRespondNotFound(t *testing.T) {
	log.Init(log.Config{
		Level:           "info",
		Format:          "console",
		IncludeSource:   false,
		RequestIDHeader: "X-Request-ID",
	})

	req := httptest.NewRequest("GET", "/notfound", nil)
	req.Header.Set("X-Request-ID", "test-req-404")
	ctx := log.ContextWithRequestID(req.Context(), "test-req-404")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	RespondNotFound(w, req, "Resource not found")

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}

	var errorResp ErrorResponse
	json.NewDecoder(w.Body).Decode(&errorResp)

	if errorResp.Code != "NOT_FOUND" {
		t.Errorf("Expected code 'NOT_FOUND', got '%s'", errorResp.Code)
	}
}

func TestStructuredErrorResponseFormat(t *testing.T) {
	log.Init(log.Config{
		Level:           "info",
		Format:          "console",
		IncludeSource:   false,
		RequestIDHeader: "X-Request-ID",
	})

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := log.ContextWithRequestID(req.Context(), "format-test-123")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	RespondErrorWithIDAndCode(w, req, "Database connection failed", "DB_ERROR", "Connection timeout after 5s")

	// Get body as string first
	body := w.Body.String()

	// Verify JSON structure is valid
	if !bytes.Contains([]byte(body), []byte("\"error\"")) {
		t.Error("Response should contain 'error' field")
	}
	if !bytes.Contains([]byte(body), []byte("\"errorId\"")) {
		t.Error("Response should contain 'errorId' field")
	}
	if !bytes.Contains([]byte(body), []byte("\"code\"")) {
		t.Error("Response should contain 'code' field")
	}
	if !bytes.Contains([]byte(body), []byte("\"time\"")) {
		t.Error("Response should contain 'time' field")
	}

	// Parse JSON to verify structure
	var errorResp ErrorResponse
	if err := json.Unmarshal([]byte(body), &errorResp); err != nil {
		t.Fatalf("Failed to parse error response JSON: %v", err)
	}

	// Verify all fields are present
	if errorResp.Error == "" {
		t.Error("Error field should not be empty")
	}
	if errorResp.ErrorID == "" {
		t.Error("ErrorID field should not be empty")
	}
	if errorResp.Code == "" {
		t.Error("Code field should not be empty")
	}
	if errorResp.Time == "" {
		t.Error("Time field should not be empty")
	}
	if errorResp.Details == "" {
		t.Error("Details field should not be empty when provided")
	}
}
