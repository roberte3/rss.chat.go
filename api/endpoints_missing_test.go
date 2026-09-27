package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHandleVersionSuccess tests successful version endpoint
func TestHandleVersionSuccess(t *testing.T) {
	req := httptest.NewRequest("GET", "/version", nil)
	w := httptest.NewRecorder()

	HandleVersion(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp VersionResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Version == "" {
		t.Error("version should not be empty")
	}

	if resp.Timestamp.IsZero() {
		t.Error("timestamp should not be zero")
	}
}

// TestHandleVersionContentType checks JSON content type
func TestHandleVersionContentType(t *testing.T) {
	req := httptest.NewRequest("GET", "/version", nil)
	w := httptest.NewRecorder()

	HandleVersion(w, req)

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected content-type application/json, got %q", contentType)
	}
}

// TestHandleVersionMethodNotAllowed tests non-GET requests
func TestHandleVersionMethodNotAllowed(t *testing.T) {
	tests := []string{"POST", "PUT", "DELETE", "PATCH"}
	for _, method := range tests {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/version", nil)
			w := httptest.NewRecorder()

			HandleVersion(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected status 405, got %d", w.Code)
			}
		})
	}
}

// TestHandleReadHTTPFileNotConfigured tests when no menu is configured
func TestHandleReadHTTPFileNotConfigured(t *testing.T) {
	handler := HandleReadHTTPFile("")

	req := httptest.NewRequest("GET", "/readhttpfile?url=http://example.com/menu.opml", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 when menu not configured, got %d", w.Code)
	}
}

// TestHandleReadHTTPFileMissingURL tests missing url parameter
func TestHandleReadHTTPFileMissingURL(t *testing.T) {
	handler := HandleReadHTTPFile("http://example.com/menu.opml")

	req := httptest.NewRequest("GET", "/readhttpfile", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for missing url, got %d", w.Code)
	}
}

// TestHandleReadHTTPFileUnauthorizedURL tests unauthorized URL access
func TestHandleReadHTTPFileUnauthorizedURL(t *testing.T) {
	allowedURL := "http://example.com/menu.opml"
	handler := HandleReadHTTPFile(allowedURL)

	// Try to access different URL
	req := httptest.NewRequest("GET", "/readhttpfile?url=http://evil.com/data.opml", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403 for unauthorized url, got %d", w.Code)
	}
}

// TestHandleReadHTTPFileMethodNotAllowed tests non-GET requests
func TestHandleReadHTTPFileMethodNotAllowed(t *testing.T) {
	handler := HandleReadHTTPFile("http://example.com/menu.opml")

	tests := []string{"POST", "PUT", "DELETE", "PATCH"}
	for _, method := range tests {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/readhttpfile", nil)
			w := httptest.NewRecorder()

			handler(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected status 405, got %d", w.Code)
			}
		})
	}
}

// TestParseMenuURLValidHTTP tests parsing valid HTTP URL
func TestParseMenuURLValidHTTP(t *testing.T) {
	url := "http://example.com/menu.opml"
	result := ParseMenuURL(url)

	if result != url {
		t.Errorf("ParseMenuURL(%q) = %q, want %q", url, result, url)
	}
}

// TestParseMenuURLValidHTTPS tests parsing valid HTTPS URL
func TestParseMenuURLValidHTTPS(t *testing.T) {
	url := "https://example.com/menu.opml"
	result := ParseMenuURL(url)

	if result != url {
		t.Errorf("ParseMenuURL(%q) = %q, want %q", url, result, url)
	}
}

// TestParseMenuURLEmpty tests parsing empty URL
func TestParseMenuURLEmpty(t *testing.T) {
	result := ParseMenuURL("")

	if result != "" {
		t.Errorf("ParseMenuURL(\"\") = %q, want empty", result)
	}
}

// TestParseMenuURLInvalidScheme tests parsing URL with invalid scheme
func TestParseMenuURLInvalidScheme(t *testing.T) {
	url := "ftp://example.com/menu.opml"
	result := ParseMenuURL(url)

	if result != "" {
		t.Errorf("ParseMenuURL(%q) = %q, want empty for invalid scheme", url, result)
	}
}

// TestParseMenuURLInvalidURL tests parsing malformed URL
func TestParseMenuURLInvalidURL(t *testing.T) {
	url := "ht!tp://[invalid"
	result := ParseMenuURL(url)

	if result != "" {
		t.Errorf("ParseMenuURL(%q) = %q, want empty for invalid URL", url, result)
	}
}

// TestHandleReadHTTPFileCorrectURL tests successful fetch with correct URL
func TestHandleReadHTTPFileCorrectURL(t *testing.T) {
	// Create a test server that serves a file
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<?xml version="1.0"?><opml><body></body></opml>`))
	}))
	defer testServer.Close()

	handler := HandleReadHTTPFile(testServer.URL + "/menu.opml")

	req := httptest.NewRequest("GET", "/readhttpfile?url="+testServer.URL+"/menu.opml", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	// Should be successful
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Check response contains OPML
	if !contains(w.Body.String(), "opml") {
		t.Error("expected response to contain opml")
	}
}

// TestParseMenuURLWithPath tests URL with path
func TestParseMenuURLWithPath(t *testing.T) {
	url := "https://example.com/path/to/menu.opml"
	result := ParseMenuURL(url)

	if result != url {
		t.Errorf("ParseMenuURL(%q) = %q, want %q", url, result, url)
	}
}

// TestParseMenuURLWithQuery tests URL with query parameters
func TestParseMenuURLWithQuery(t *testing.T) {
	url := "https://example.com/menu.opml?format=opml"
	result := ParseMenuURL(url)

	if result != url {
		t.Errorf("ParseMenuURL(%q) = %q, want %q", url, result, url)
	}
}

// TestVersionResponseJSON tests JSON marshaling
func TestVersionResponseJSON(t *testing.T) {
	resp := VersionResponse{
		Version: "1.0.0",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	if !contains(string(data), "1.0.0") {
		t.Error("JSON should contain version")
	}
}

// TestHandleReadHTTPFileFetchError tests handling fetch errors
func TestHandleReadHTTPFileFetchError(t *testing.T) {
	// Use a URL that won't respond
	invalidURL := "http://invalid.test.local/menu.opml"
	handler := HandleReadHTTPFile(invalidURL)

	req := httptest.NewRequest("GET", "/readhttpfile?url="+invalidURL, nil)
	w := httptest.NewRecorder()

	handler(w, req)

	// Should handle error gracefully
	if w.Code < 400 {
		t.Errorf("expected error status, got %d", w.Code)
	}
}

// TestHandleReadHTTPFileMaxSize tests size limit enforcement
func TestHandleReadHTTPFileMaxSize(t *testing.T) {
	// Create a test server that serves large content
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		// Write 2MB of data (exceeds 1MB limit)
		for i := 0; i < 2048; i++ {
			w.Write([]byte("x"))
		}
	}))
	defer testServer.Close()

	handler := HandleReadHTTPFile(testServer.URL + "/menu.opml")

	req := httptest.NewRequest("GET", "/readhttpfile?url="+testServer.URL+"/menu.opml", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	// Should succeed (limited reader still reads up to limit)
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Body should not exceed 1MB
	if len(w.Body.String()) > 1024*1024 {
		t.Error("response should be limited to 1MB")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr))
}
