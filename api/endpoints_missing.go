package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"time"

	applog "github.com/roberte3/rss.chat.go/log"
)

// VersionResponse is the response for the /version endpoint
type VersionResponse struct {
	Version   string    `json:"version"`
	Timestamp time.Time `json:"timestamp"`
}

// HandleVersion returns the application version
func HandleVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Version is defined in main package, so we pass it as a parameter
	// For now, return a hardcoded version - will be passed from main.go
	resp := VersionResponse{
		Version:   "0.7.0",
		Timestamp: time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// HandleReadHTTPFile serves menu OPML with security restrictions
// Only fetches from config.urlMenuOpml to prevent SSRF attacks
func HandleReadHTTPFile(urlMenuOpml string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Check if menu OPML is configured
		if urlMenuOpml == "" {
			// No menu configured, return 404
			http.NotFound(w, r)
			return
		}

		logger := applog.WithContext(r.Context())

		// Get the URL parameter
		urlParam := r.URL.Query().Get("url")
		if urlParam == "" {
			logger.WarnContext(r.Context(),
				"readhttpfile: missing url parameter",
				"path", r.URL.Path,
			)
			http.Error(w, "Missing url parameter", http.StatusBadRequest)
			return
		}

		// Security check: only allow fetching from configured urlMenuOpml
		if urlParam != urlMenuOpml {
			logger.WarnContext(r.Context(),
				"readhttpfile: attempted unauthorized fetch",
				"requested_url", urlParam,
				"allowed_url", urlMenuOpml,
				"remote_addr", r.RemoteAddr,
			)
			http.Error(w, "Unauthorized", http.StatusForbidden)
			return
		}

		logger.DebugContext(r.Context(),
			"readhttpfile: fetching menu",
			"url", urlMenuOpml,
		)

		// Fetch the file with timeout
		client := &http.Client{
			Timeout: 10 * time.Second,
		}

		resp, err := client.Get(urlMenuOpml)
		if err != nil {
			logger.ErrorContext(r.Context(),
				"readhttpfile: fetch failed",
				"url", urlMenuOpml,
				"err", err,
			)
			http.Error(w, "Failed to fetch file", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		// Check response status
		if resp.StatusCode != http.StatusOK {
			logger.WarnContext(r.Context(),
				"readhttpfile: fetch returned non-200 status",
				"url", urlMenuOpml,
				"status", resp.StatusCode,
			)
			http.Error(w, "Failed to fetch file", http.StatusInternalServerError)
			return
		}

		// Copy response headers (preserve Content-Type, etc.)
		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}

		// Limit response size to 1MB to prevent memory exhaustion
		limitedReader := io.LimitReader(resp.Body, 1024*1024)

		// Write the file content
		_, err = io.Copy(w, limitedReader)
		if err != nil {
			logger.ErrorContext(r.Context(),
				"readhttpfile: copy failed",
				"url", urlMenuOpml,
				"err", err,
			)
		}

		logger.DebugContext(r.Context(),
			"readhttpfile: fetch completed",
			"url", urlMenuOpml,
		)
	}
}

// ParseMenuURL safely parses a menu URL from config
// Returns empty string if URL is invalid
func ParseMenuURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}

	// Parse the URL to ensure it's valid
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	// Only allow http and https schemes
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return ""
	}

	return rawURL
}
