package oteladapters_test

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/log/noop"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/oteladapters"
)

func Test_NewSlogBridgeLogger_Construction(t *testing.T) {
	logger := oteladapters.NewSlogBridgeLogger("test")
	assert.NotNil(t, logger, "NewSlogBridgeLogger should return non-nil logger")
}

func Test_NewSlogBridgeLoggerWithHandler_Construction(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)

	logger := oteladapters.NewSlogBridgeLoggerWithHandler(handler)
	assert.NotNil(t, logger, "NewSlogBridgeLoggerWithHandler should return non-nil logger")
}

func Test_SlogBridgeLogger_AllLevels(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug, // Capture all levels
	})

	logger := oteladapters.NewSlogBridgeLoggerWithHandler(handler)
	ctx := context.Background()

	// Test all log levels
	logger.DebugContext(ctx, "debug message", "level", "debug")
	logger.InfoContext(ctx, "info message", "level", "info")
	logger.WarnContext(ctx, "warn message", "level", "warn")
	logger.ErrorContext(ctx, "error message", "level", "error")

	output := buf.String()

	// Verify all messages were logged
	assert.Contains(t, output, "debug message", "Debug message should be logged")
	assert.Contains(t, output, "info message", "Info message should be logged")
	assert.Contains(t, output, "warn message", "Warn message should be logged")
	assert.Contains(t, output, "error message", "Error message should be logged")

	// Verify level information
	assert.Contains(t, output, `"level":"debug"`, "Debug level should be present")
	assert.Contains(t, output, `"level":"info"`, "Info level should be present")
	assert.Contains(t, output, `"level":"warn"`, "Warn level should be present")
	assert.Contains(t, output, `"level":"error"`, "Error level should be present")
}

func Test_SlogBridgeLogger_WithAttributes(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)

	logger := oteladapters.NewSlogBridgeLoggerWithHandler(handler)
	ctx := context.Background()

	// Log with various attribute types
	logger.InfoContext(ctx, "test message",
		"string_attr", "value1",
		"int_attr", 42,
		"float_attr", 3.14,
		"bool_attr", true,
	)

	output := buf.String()

	// Verify a message and attributes
	assert.Contains(t, output, "test message", "Message should be logged")
	assert.Contains(t, output, `"string_attr":"value1"`, "String attribute should be present")
	assert.Contains(t, output, `"int_attr":42`, "Int attribute should be present")
	assert.Contains(t, output, `"float_attr":3.14`, "Float attribute should be present")
	assert.Contains(t, output, `"bool_attr":true`, "Bool attribute should be present")
}

func Test_NewOTelLogger_Construction(t *testing.T) {
	// Use a noop logger for simple construction testing
	otelLogger := noop.NewLoggerProvider().Logger("test")

	logger := oteladapters.NewOTelLogger(otelLogger)
	assert.NotNil(t, logger, "NewOTelLogger should return non-nil logger")
}

func Test_OTelLogger_AllLevels(t *testing.T) {
	// Use noop logger - we just want to verify methods don't panic
	otelLogger := noop.NewLoggerProvider().Logger("test")
	logger := oteladapters.NewOTelLogger(otelLogger)
	ctx := context.Background()

	// Test all log levels - should not panic
	assert.NotPanics(t, func() {
		logger.DebugContext(ctx, "debug message", "test_key", "debug_value")
	}, "DebugContext should not panic")

	assert.NotPanics(t, func() {
		logger.InfoContext(ctx, "info message", "test_key", "info_value")
	}, "InfoContext should not panic")

	assert.NotPanics(t, func() {
		logger.WarnContext(ctx, "warn message", "test_key", "warn_value")
	}, "WarnContext should not panic")

	assert.NotPanics(t, func() {
		logger.ErrorContext(ctx, "error message", "test_key", "error_value")
	}, "ErrorContext should not panic")
}

func Test_OTelLogger_ArgumentHandling(t *testing.T) {
	otelLogger := noop.NewLoggerProvider().Logger("test")
	logger := oteladapters.NewOTelLogger(otelLogger)
	ctx := context.Background()

	// Test various argument scenarios - should not panic
	assert.NotPanics(t, func() {
		logger.InfoContext(ctx, "test message",
			"string", "text_value",
			"number", 123,
			"float", 45.67,
			"boolean", false,
		)
	}, "Multiple args should not panic")

	assert.NotPanics(t, func() {
		logger.InfoContext(ctx, "test message", "key1", "value1", "key2")
	}, "Odd number of args should not panic")

	assert.NotPanics(t, func() {
		logger.InfoContext(ctx, "simple message")
	}, "No additional args should not panic")
}
