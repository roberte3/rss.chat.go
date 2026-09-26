package db

import (
	"context"
	"fmt"
	"time"

	applog "github.com/roberte3/rss.chat.go/log"
)

// LogQuery logs a database query with timing information.
// Call this at the end of a database operation to log duration and status.
func LogQuery(ctx context.Context, operation string, table string, startTime time.Time, err error) {
	duration := time.Since(startTime)
	durationMs := float64(duration.Milliseconds())
	logger := applog.WithContext(ctx)

	// Log slow queries as warnings
	if durationMs > 100 {
		logger.WarnContext(ctx,
			"slow database query",
			"operation", operation,
			"table", table,
			"duration_ms", fmt.Sprintf("%.2f", durationMs),
			"err", err,
		)
		return
	}

	// Log errors
	if err != nil {
		logger.ErrorContext(ctx,
			"database query failed",
			"operation", operation,
			"table", table,
			"duration_ms", fmt.Sprintf("%.2f", durationMs),
			"err", err,
		)
		return
	}

	// Log at debug level for successful queries
	logger.DebugContext(ctx,
		"database query completed",
		"operation", operation,
		"table", table,
		"duration_ms", fmt.Sprintf("%.2f", durationMs),
	)
}

// LogTransaction logs transaction status (start/commit/rollback)
func LogTransaction(ctx context.Context, action string, err error) {
	logger := applog.WithContext(ctx)

	if err != nil {
		logger.ErrorContext(ctx,
			"transaction failed",
			"action", action,
			"err", err,
		)
		return
	}

	logger.DebugContext(ctx,
		"transaction "+action,
		"action", action,
	)
}

// LogConstraintViolation logs when a unique constraint or other constraint fails
func LogConstraintViolation(ctx context.Context, constraint string, details string, err error) {
	logger := applog.WithContext(ctx)
	logger.WarnContext(ctx,
		"constraint violation",
		"constraint", constraint,
		"details", details,
		"err", err,
	)
}
