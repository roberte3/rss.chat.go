package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	applog "github.com/roberte3/rss.chat.go/log"
)

// ErrorResponse represents a structured error response sent to clients.
type ErrorResponse struct {
	Error   string `json:"error"`            // Human-readable error message
	ErrorID string `json:"errorId"`          // Unique error ID for tracing (err-{requestID})
	Code    string `json:"code,omitempty"`   // Machine-readable error code
	Details string `json:"details,omitempty"` // Additional error details
	Time    string `json:"time"`             // Timestamp when error occurred
}

// RespondErrorWithID sends a structured JSON error response with an error ID.
// The error ID is derived from the request ID in the context.
func RespondErrorWithID(w http.ResponseWriter, r *http.Request, message string) {
	RespondErrorWithIDAndCode(w, r, message, "ERROR", "")
}

// RespondErrorWithIDAndCode sends a structured JSON error response with an error ID and code.
func RespondErrorWithIDAndCode(w http.ResponseWriter, r *http.Request, message string, code string, details string) {
	ctx := r.Context()
	logger := applog.WithContext(ctx)

	// Get request ID from context for error tracking
	errorID := getErrorIDFromContext(ctx)

	// Log the error with context
	logger.ErrorContext(ctx,
		"error response sent",
		"error_id", errorID,
		"code", code,
		"message", message,
		"path", r.URL.Path,
		"method", r.Method,
	)

	errorResp := ErrorResponse{
		Error:   message,
		ErrorID: errorID,
		Code:    code,
		Details: details,
		Time:    time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	json.NewEncoder(w).Encode(errorResp)
}

// RespondValidationError sends a structured JSON error response for validation failures.
func RespondValidationError(w http.ResponseWriter, r *http.Request, field string, reason string) {
	ctx := r.Context()
	logger := applog.WithContext(ctx)

	errorID := getErrorIDFromContext(ctx)
	message := fmt.Sprintf("Validation error: %s (%s)", field, reason)

	logger.WarnContext(ctx,
		"validation error response sent",
		"error_id", errorID,
		"field", field,
		"reason", reason,
	)

	errorResp := ErrorResponse{
		Error:   message,
		ErrorID: errorID,
		Code:    "VALIDATION_ERROR",
		Details: fmt.Sprintf("Field: %s, Reason: %s", field, reason),
		Time:    time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(errorResp)
}

// RespondAuthError sends a structured JSON error response for authentication failures.
func RespondAuthError(w http.ResponseWriter, r *http.Request, message string) {
	ctx := r.Context()
	logger := applog.WithContext(ctx)

	errorID := getErrorIDFromContext(ctx)

	logger.WarnContext(ctx,
		"auth error response sent",
		"error_id", errorID,
		"message", message,
	)

	errorResp := ErrorResponse{
		Error:   message,
		ErrorID: errorID,
		Code:    "AUTH_ERROR",
		Time:    time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(errorResp)
}

// RespondNotFound sends a structured JSON 404 response.
func RespondNotFound(w http.ResponseWriter, r *http.Request, message string) {
	ctx := r.Context()
	errorID := getErrorIDFromContext(ctx)

	errorResp := ErrorResponse{
		Error:   message,
		ErrorID: errorID,
		Code:    "NOT_FOUND",
		Time:    time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(errorResp)
}

// getErrorIDFromContext extracts the request ID from context and formats it as an error ID.
// If no request ID exists, returns a generic error ID.
func getErrorIDFromContext(ctx interface{}) string {
	// Try to convert to context.Context
	if actx, ok := ctx.(context.Context); ok {
		requestID := applog.RequestIDFromContext(actx)
		if requestID != "" {
			return "err-" + requestID
		}
	}
	return "err-unknown"
}
