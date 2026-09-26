// Package log provides structured logging for the application.
package log

import (
	"context"
	"log/slog"
	"os"
)

var (
	// Root logger used throughout the application
	logger *slog.Logger
)

// Config holds logging configuration
type Config struct {
	Level           string // debug, info, warn, error
	Format          string // json or console
	IncludeSource   bool
	RequestIDHeader string
}

// Init initializes the global logger with the given configuration.
func Init(cfg Config) error {
	var level slog.Level
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: cfg.IncludeSource,
	}

	var handler slog.Handler
	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	logger = slog.New(handler)
	slog.SetDefault(logger)

	return nil
}

// WithContext returns a logger with the given context (e.g., containing request ID).
func WithContext(ctx context.Context) *slog.Logger {
	if logger == nil {
		return slog.Default()
	}

	// Extract request ID from context if present
	if reqID, ok := ctx.Value(contextKeyRequestID).(string); ok {
		return logger.With("request_id", reqID)
	}

	return logger
}

// Logger returns the global logger.
func Logger() *slog.Logger {
	if logger == nil {
		return slog.Default()
	}
	return logger
}

// Context key for request ID
type contextKey string

const contextKeyRequestID contextKey = "request_id"

// ContextWithRequestID returns a context with the request ID attached.
func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, contextKeyRequestID, requestID)
}

// RequestIDFromContext extracts the request ID from a context.
func RequestIDFromContext(ctx context.Context) string {
	if reqID, ok := ctx.Value(contextKeyRequestID).(string); ok {
		return reqID
	}
	return ""
}
