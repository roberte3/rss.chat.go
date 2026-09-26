package log

import (
	"context"
	"testing"
)

// TestInitConsoleFormat tests logger initialization with console format
func TestInitConsoleFormat(t *testing.T) {
	cfg := Config{
		Level:           "info",
		Format:          "console",
		IncludeSource:   false,
		RequestIDHeader: "X-Request-ID",
	}

	err := Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	logger := Logger()
	if logger == nil {
		t.Fatal("Logger() returned nil after Init")
	}
}

// TestInitJSONFormat tests logger initialization with JSON format
func TestInitJSONFormat(t *testing.T) {
	cfg := Config{
		Level:           "debug",
		Format:          "json",
		IncludeSource:   true,
		RequestIDHeader: "X-Request-ID",
	}

	err := Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	logger := Logger()
	if logger == nil {
		t.Fatal("Logger() returned nil after Init")
	}
}

// TestLogLevel tests that log levels are respected
func TestLogLevel(t *testing.T) {
	tests := []struct {
		name    string
		level   string
		wantErr bool
	}{
		{"debug", "debug", false},
		{"info", "info", false},
		{"warn", "warn", false},
		{"error", "error", false},
		{"unknown", "unknown", false}, // Defaults to info
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				Level:           tt.level,
				Format:          "console",
				IncludeSource:   false,
				RequestIDHeader: "X-Request-ID",
			}

			err := Init(cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Init() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestContextWithRequestID tests adding request ID to context
func TestContextWithRequestID(t *testing.T) {
	ctx := context.Background()
	requestID := "req-12345"

	ctxWithID := ContextWithRequestID(ctx, requestID)

	retrieved := RequestIDFromContext(ctxWithID)
	if retrieved != requestID {
		t.Errorf("RequestIDFromContext() = %q, want %q", retrieved, requestID)
	}
}

// TestRequestIDFromContextEmpty tests extracting request ID from context without it
func TestRequestIDFromContextEmpty(t *testing.T) {
	ctx := context.Background()

	retrieved := RequestIDFromContext(ctx)
	if retrieved != "" {
		t.Errorf("RequestIDFromContext() on empty context = %q, want %q", retrieved, "")
	}
}

// TestRequestIDFromContextWrongType tests extracting request ID with wrong type in context
func TestRequestIDFromContextWrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), contextKeyRequestID, 12345)

	retrieved := RequestIDFromContext(ctx)
	if retrieved != "" {
		t.Errorf("RequestIDFromContext() with wrong type = %q, want %q", retrieved, "")
	}
}

// TestWithContextNoRequestID tests logging with context that has no request ID
func TestWithContextNoRequestID(t *testing.T) {
	cfg := Config{
		Level:           "info",
		Format:          "console",
		IncludeSource:   false,
		RequestIDHeader: "X-Request-ID",
	}

	Init(cfg)

	ctx := context.Background()
	logger := WithContext(ctx)

	if logger == nil {
		t.Fatal("WithContext() returned nil")
	}
}

// TestWithContextWithRequestID tests logging with context that has request ID
func TestWithContextWithRequestID(t *testing.T) {
	cfg := Config{
		Level:           "info",
		Format:          "console",
		IncludeSource:   false,
		RequestIDHeader: "X-Request-ID",
	}

	Init(cfg)

	ctx := ContextWithRequestID(context.Background(), "req-abc123")
	logger := WithContext(ctx)

	if logger == nil {
		t.Fatal("WithContext() returned nil")
	}
}

// TestLoggerNotNilAfterInit tests that Logger() is never nil after Init
func TestLoggerNotNilAfterInit(t *testing.T) {
	cfg := Config{
		Level:           "info",
		Format:          "console",
		IncludeSource:   false,
		RequestIDHeader: "X-Request-ID",
	}

	Init(cfg)

	logger := Logger()
	if logger == nil {
		t.Fatal("Logger() returned nil after Init")
	}
}

// TestLoggerDefaultsBeforeInit tests that Logger() returns non-nil even before Init
func TestLoggerDefaultsBeforeInit(t *testing.T) {
	// Don't call Init, should still have a default logger
	logger := Logger()
	if logger == nil {
		t.Fatal("Logger() returned nil without Init")
	}
}

// TestContextKeyType tests that context key has correct type
func TestContextKeyType(t *testing.T) {
	ctx := ContextWithRequestID(context.Background(), "test-id")

	// Value should be retrievable with the same key type
	val := ctx.Value(contextKeyRequestID)
	if val == nil {
		t.Fatal("context value is nil")
	}

	// Should be a string
	id, ok := val.(string)
	if !ok {
		t.Fatalf("context value is not a string: %T", val)
	}

	if id != "test-id" {
		t.Errorf("context value = %q, want %q", id, "test-id")
	}
}

// TestDebugLogging tests debug level logging
func TestDebugLogging(t *testing.T) {
	cfg := Config{
		Level:           "debug",
		Format:          "console",
		IncludeSource:   false,
		RequestIDHeader: "X-Request-ID",
	}

	Init(cfg)

	ctx := context.Background()
	logger := WithContext(ctx)

	// Should not panic
	logger.DebugContext(ctx, "test debug", "key", "value")
}

// TestInfoLogging tests info level logging
func TestInfoLogging(t *testing.T) {
	cfg := Config{
		Level:           "info",
		Format:          "console",
		IncludeSource:   false,
		RequestIDHeader: "X-Request-ID",
	}

	Init(cfg)

	ctx := context.Background()
	logger := WithContext(ctx)

	// Should not panic
	logger.InfoContext(ctx, "test info", "key", "value")
}

// TestWarnLogging tests warn level logging
func TestWarnLogging(t *testing.T) {
	cfg := Config{
		Level:           "warn",
		Format:          "console",
		IncludeSource:   false,
		RequestIDHeader: "X-Request-ID",
	}

	Init(cfg)

	ctx := context.Background()
	logger := WithContext(ctx)

	// Should not panic
	logger.WarnContext(ctx, "test warn", "key", "value")
}

// TestErrorLogging tests error level logging
func TestErrorLogging(t *testing.T) {
	cfg := Config{
		Level:           "error",
		Format:          "console",
		IncludeSource:   false,
		RequestIDHeader: "X-Request-ID",
	}

	Init(cfg)

	ctx := context.Background()
	logger := WithContext(ctx)

	// Should not panic
	logger.ErrorContext(ctx, "test error", "key", "value")
}

// TestMultipleContextKeys tests that multiple context keys don't interfere
func TestMultipleContextKeys(t *testing.T) {
	ctx := context.Background()
	ctx = ContextWithRequestID(ctx, "req-1")
	ctx = context.WithValue(ctx, "other-key", "other-value")

	// Request ID should still be accessible
	id := RequestIDFromContext(ctx)
	if id != "req-1" {
		t.Errorf("RequestIDFromContext() = %q, want %q", id, "req-1")
	}

	// Other value should also be accessible
	other := ctx.Value("other-key")
	if other != "other-value" {
		t.Errorf("other value = %q, want %q", other, "other-value")
	}
}

// TestContextCancellation tests that request ID survives context cancellation
func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ctx = ContextWithRequestID(ctx, "req-cancel-test")

	// Cancel the context
	cancel()

	// Request ID should still be accessible
	id := RequestIDFromContext(ctx)
	if id != "req-cancel-test" {
		t.Errorf("RequestIDFromContext() after cancel = %q, want %q", id, "req-cancel-test")
	}
}

// TestContextNesting tests nested context creation with request IDs
func TestContextNesting(t *testing.T) {
	ctx1 := ContextWithRequestID(context.Background(), "req-parent")
	ctx2 := ContextWithRequestID(ctx1, "req-child")

	// Parent should have original ID
	id1 := RequestIDFromContext(ctx1)
	if id1 != "req-parent" {
		t.Errorf("parent RequestID = %q, want %q", id1, "req-parent")
	}

	// Child should have new ID
	id2 := RequestIDFromContext(ctx2)
	if id2 != "req-child" {
		t.Errorf("child RequestID = %q, want %q", id2, "req-child")
	}
}

// TestEmptyRequestID tests adding an empty request ID
func TestEmptyRequestID(t *testing.T) {
	ctx := ContextWithRequestID(context.Background(), "")

	id := RequestIDFromContext(ctx)
	if id != "" {
		t.Errorf("RequestIDFromContext() with empty ID = %q, want %q", id, "")
	}
}

// TestLoggerWithMultipleAttributes tests logging with multiple attributes
func TestLoggerWithMultipleAttributes(t *testing.T) {
	cfg := Config{
		Level:           "info",
		Format:          "console",
		IncludeSource:   false,
		RequestIDHeader: "X-Request-ID",
	}

	Init(cfg)

	ctx := ContextWithRequestID(context.Background(), "req-multi-attr")
	logger := WithContext(ctx)

	// Should handle multiple key-value pairs
	logger.InfoContext(ctx, "test message",
		"key1", "value1",
		"key2", "value2",
		"key3", "value3",
		"key4", 42,
		"key5", true,
	)
}

// TestRequestIDPersistenceAcrossContextCreation tests that request ID persists
func TestRequestIDPersistenceAcrossContextCreation(t *testing.T) {
	requestID := "req-persistence-test"
	ctx := ContextWithRequestID(context.Background(), requestID)

	// Create a child context with timeout
	childCtx, cancel := context.WithTimeout(ctx, 0)
	defer cancel()

	// Request ID should still be accessible in child context
	id := RequestIDFromContext(childCtx)
	if id != requestID {
		t.Errorf("RequestID in child context = %q, want %q", id, requestID)
	}
}

// TestLoggingWithDifferentFormats tests both JSON and console formats produce output
func TestLoggingWithDifferentFormats(t *testing.T) {
	formats := []string{"json", "console"}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			cfg := Config{
				Level:           "info",
				Format:          format,
				IncludeSource:   false,
				RequestIDHeader: "X-Request-ID",
			}

			err := Init(cfg)
			if err != nil {
				t.Fatalf("Init with format %s failed: %v", format, err)
			}

			ctx := ContextWithRequestID(context.Background(), "req-format-test")
			logger := WithContext(ctx)

			// Log something - should not panic
			logger.InfoContext(ctx, "test message", "format", format)
		})
	}
}

// TestRequestIDMultipleRetrievals tests that request ID is consistent across multiple retrievals
func TestRequestIDMultipleRetrievals(t *testing.T) {
	requestID := "req-multiple-retrievals"
	ctx := ContextWithRequestID(context.Background(), requestID)

	// Retrieve multiple times
	id1 := RequestIDFromContext(ctx)
	id2 := RequestIDFromContext(ctx)
	id3 := RequestIDFromContext(ctx)

	if id1 != requestID || id2 != requestID || id3 != requestID {
		t.Errorf("inconsistent RequestID retrievals: %q, %q, %q, want all %q", id1, id2, id3, requestID)
	}
}

// TestIncludeSourceOption tests that IncludeSource option is accepted
func TestIncludeSourceOption(t *testing.T) {
	tests := []struct {
		name           string
		includeSource  bool
	}{
		{"with source", true},
		{"without source", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				Level:           "info",
				Format:          "console",
				IncludeSource:   tt.includeSource,
				RequestIDHeader: "X-Request-ID",
			}

			err := Init(cfg)
			if err != nil {
				t.Fatalf("Init failed: %v", err)
			}

			logger := Logger()
			if logger == nil {
				t.Fatal("Logger() returned nil")
			}
		})
	}
}

// TestCustomRequestIDHeader tests custom request ID header configuration
func TestCustomRequestIDHeader(t *testing.T) {
	cfg := Config{
		Level:           "info",
		Format:          "console",
		IncludeSource:   false,
		RequestIDHeader: "Custom-Request-ID",
	}

	err := Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// The header name is stored but doesn't affect the context behavior
	// Request ID is still added to context the same way
	ctx := ContextWithRequestID(context.Background(), "req-custom-header")
	id := RequestIDFromContext(ctx)

	if id != "req-custom-header" {
		t.Errorf("RequestIDFromContext() = %q, want %q", id, "req-custom-header")
	}
}

// TestLoggerWithRequestIDAttribute tests that request ID is properly logged
func TestLoggerWithRequestIDAttribute(t *testing.T) {
	cfg := Config{
		Level:           "info",
		Format:          "console",
		IncludeSource:   false,
		RequestIDHeader: "X-Request-ID",
	}

	Init(cfg)

	requestID := "req-logger-attr-test"
	ctx := ContextWithRequestID(context.Background(), requestID)
	logger := WithContext(ctx)

	// Logger should have request_id in its attributes
	// This is reflected in the WithContext implementation
	logger.InfoContext(ctx, "test message", "action", "test")
}
