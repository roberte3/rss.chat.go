package publish

import (
	"context"
	"fmt"
	"time"

	applog "github.com/roberte3/rss.chat.go/log"
)

// LogFeedGeneration logs feed generation with timing information.
func LogFeedGeneration(ctx context.Context, feedType string, identifier string, startTime time.Time, err error) {
	duration := time.Since(startTime)
	durationMs := float64(duration.Milliseconds())
	logger := applog.WithContext(ctx)

	if err != nil {
		logger.ErrorContext(ctx,
			"feed generation failed",
			"feed_type", feedType,
			"identifier", identifier,
			"duration_ms", fmt.Sprintf("%.2f", durationMs),
			"err", err,
		)
		return
	}

	logger.DebugContext(ctx,
		"feed generated",
		"feed_type", feedType,
		"identifier", identifier,
		"duration_ms", fmt.Sprintf("%.2f", durationMs),
	)
}

// LogWebSubPing logs a WebSub hub ping attempt.
func LogWebSubPing(ctx context.Context, feedURL string, err error) {
	logger := applog.WithContext(ctx)

	if err != nil {
		logger.WarnContext(ctx,
			"websub ping failed",
			"feed_url", feedURL,
			"err", err,
		)
		return
	}

	logger.DebugContext(ctx,
		"websub ping sent",
		"feed_url", feedURL,
	)
}

// LogFeedsUpdate logs a feeds update operation (post write, reply, like).
func LogFeedsUpdate(ctx context.Context, operation string, details map[string]interface{}, err error) {
	logger := applog.WithContext(ctx)
	args := []interface{}{"operation", operation}
	for k, v := range details {
		args = append(args, k, v)
	}

	if err != nil {
		args = append(args, "err", err)
		logger.WarnContext(ctx, "feeds update failed", args...)
		return
	}

	logger.DebugContext(ctx, "feeds updated", args...)
}

// LogBackfillOperation logs a backfill operation (feeds, comments).
func LogBackfillOperation(ctx context.Context, operationType string, count int, startTime time.Time, err error) {
	duration := time.Since(startTime)
	durationMs := float64(duration.Milliseconds())
	logger := applog.WithContext(ctx)

	if err != nil {
		logger.WarnContext(ctx,
			"backfill operation failed",
			"type", operationType,
			"duration_ms", fmt.Sprintf("%.2f", durationMs),
			"err", err,
		)
		return
	}

	logger.InfoContext(ctx,
		"backfill operation completed",
		"type", operationType,
		"count", count,
		"duration_ms", fmt.Sprintf("%.2f", durationMs),
	)
}
