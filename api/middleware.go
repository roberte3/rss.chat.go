package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/roberte3/rss.chat.go/log"
)

// RequestLogger is middleware that logs HTTP requests and responses with request IDs,
// and collects Prometheus metrics.
type RequestLogger struct {
	next            http.Handler
	requestIDHeader string
	logger          *slog.Logger
	metrics         *Metrics
	excludePaths    map[string]bool
}

// NewRequestLogger creates a new request logging middleware.
func NewRequestLogger(next http.Handler, requestIDHeader string) *RequestLogger {
	return &RequestLogger{
		next:            next,
		requestIDHeader: requestIDHeader,
		logger:          log.Logger(),
		metrics:         GetMetrics(),
		excludePaths: map[string]bool{
			"/health":  true,
			"/healthz": true,
			"/ready":   true,
			"/metrics": true,
		},
	}
}

// ServeHTTP implements the http.Handler interface.
func (rl *RequestLogger) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Skip logging and metrics for health check endpoints
	if rl.excludePaths[r.URL.Path] {
		rl.next.ServeHTTP(w, r)
		return
	}

	// Get or generate request ID
	requestID := r.Header.Get(rl.requestIDHeader)
	if requestID == "" {
		requestID = uuid.New().String()
	}

	// Add request ID to context
	r = r.WithContext(log.ContextWithRequestID(r.Context(), requestID))

	// Wrap response writer to capture status
	wrapped := &responseLogger{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}

	// Log request start and record request count
	startTime := time.Now()
	rl.logger.InfoContext(
		r.Context(),
		"request started",
		"method", r.Method,
		"path", r.URL.Path,
		"query", r.URL.RawQuery,
		"remote_addr", r.RemoteAddr,
	)

	// Increment request counter
	rl.metrics.HTTPRequestsTotal.Inc()

	// Handle the request
	rl.next.ServeHTTP(wrapped, r)

	// Record metrics
	duration := time.Since(startTime)
	durationSeconds := duration.Seconds()

	// Record request duration histogram
	rl.metrics.HTTPRequestDuration.Observe(durationSeconds)

	// Count errors if status >= 400
	if wrapped.statusCode >= 400 {
		rl.metrics.HTTPErrorsTotal.Inc()
	}

	// Log request completion
	contextLogger := log.WithContext(r.Context())
	durationMs := fmt.Sprintf("%.2f", float64(duration.Milliseconds()))

	switch logLevel(wrapped.statusCode) {
	case slog.LevelWarn:
		contextLogger.WarnContext(
			r.Context(),
			"request completed",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.statusCode,
			"duration_ms", durationMs,
			"bytes_written", wrapped.bytesWritten,
		)
	case slog.LevelError:
		contextLogger.ErrorContext(
			r.Context(),
			"request completed",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.statusCode,
			"duration_ms", durationMs,
			"bytes_written", wrapped.bytesWritten,
		)
	default:
		contextLogger.InfoContext(
			r.Context(),
			"request completed",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.statusCode,
			"duration_ms", durationMs,
			"bytes_written", wrapped.bytesWritten,
		)
	}
}

// responseLogger wraps http.ResponseWriter to capture status code and bytes written.
type responseLogger struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

// WriteHeader captures the HTTP status code.
func (rl *responseLogger) WriteHeader(code int) {
	rl.statusCode = code
	rl.ResponseWriter.WriteHeader(code)
}

// Write captures bytes written.
func (rl *responseLogger) Write(b []byte) (int, error) {
	n, err := rl.ResponseWriter.Write(b)
	rl.bytesWritten += n
	return n, err
}

// logLevel returns the appropriate log level based on HTTP status code.
func logLevel(statusCode int) slog.Level {
	switch {
	case statusCode < 400:
		return slog.LevelInfo
	case statusCode < 500:
		return slog.LevelWarn
	default:
		return slog.LevelError
	}
}
