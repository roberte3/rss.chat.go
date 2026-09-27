package api

import (
	"context"
	"net/http"
	"time"

	"github.com/roberte3/rss.chat.go/db"
	applog "github.com/roberte3/rss.chat.go/log"
)

// LogAuthFailure logs an authentication failure without exposing sensitive details.
func LogAuthFailure(r *http.Request, reason string) {
	logger := applog.WithContext(r.Context())
	logger.WarnContext(r.Context(),
		"authentication failed",
		"reason", reason,
		"path", r.URL.Path,
		"method", r.Method,
	)
}

// LogAuthSuccess logs a successful authentication at debug level.
func LogAuthSuccess(ctx context.Context, screenname string) {
	logger := applog.WithContext(ctx)
	logger.DebugContext(ctx,
		"authentication successful",
		"screenname", screenname,
	)
}

// LogValidationError logs when request validation fails.
func LogValidationError(r *http.Request, field string, reason string) {
	logger := applog.WithContext(r.Context())
	logger.WarnContext(r.Context(),
		"validation error",
		"field", field,
		"reason", reason,
		"path", r.URL.Path,
		"method", r.Method,
	)
}

// LogOperationStart logs the start of a major operation (post creation, update, etc).
func LogOperationStart(ctx context.Context, operation string, details map[string]interface{}) {
	logger := applog.WithContext(ctx)
	args := []interface{}{"operation", operation}
	for k, v := range details {
		args = append(args, k, v)
	}
	logger.DebugContext(ctx, "operation started", args...)
}

// LogOperationComplete logs the completion of a major operation.
func LogOperationComplete(ctx context.Context, operation string, details map[string]interface{}) {
	logger := applog.WithContext(ctx)
	args := []interface{}{"operation", operation}
	for k, v := range details {
		args = append(args, k, v)
	}
	logger.InfoContext(ctx, "operation completed", args...)
}

// LogOperationError logs when a major operation fails.
func LogOperationError(ctx context.Context, operation string, err error, details map[string]interface{}) {
	logger := applog.WithContext(ctx)
	args := []interface{}{"operation", operation, "err", err}
	for k, v := range details {
		args = append(args, k, v)
	}
	logger.ErrorContext(ctx, "operation failed", args...)
}

// LogFeatureUsage logs when a feature is used (mentions extracted, hashtags created, etc).
func LogFeatureUsage(ctx context.Context, feature string, count int) {
	logger := applog.WithContext(ctx)
	logger.DebugContext(ctx,
		"feature usage",
		"feature", feature,
		"count", count,
	)
}

// LogWarning logs a non-critical warning during processing.
func LogWarning(ctx context.Context, message string, details ...interface{}) {
	logger := applog.WithContext(ctx)
	logger.WarnContext(ctx, message, details...)
}

// LogQueryWithMetrics logs a database query and records metrics.
// This wraps db.LogQuery to also record Prometheus metrics.
func LogQueryWithMetrics(ctx context.Context, operation string, table string, startTime time.Time, err error) {
	// Log the query (with standard logging)
	db.LogQuery(ctx, operation, table, startTime, err)

	// Record metrics
	metrics := GetMetrics()
	duration := time.Since(startTime)
	durationSeconds := duration.Seconds()

	// Increment query counter
	metrics.DBQueriesTotal.Inc()

	// Record query duration histogram
	metrics.DBQueryDuration.Observe(durationSeconds)
}
