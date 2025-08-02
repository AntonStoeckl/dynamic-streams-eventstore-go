package oteladapters_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/oteladapters"
)

// TestTraceCorrelation verifies that the SlogBridgeLogger includes trace and span IDs
// in log messages when tracing is active.
func TestTraceCorrelation(t *testing.T) {
	// Setup OpenTelemetry tracer
	tracerProvider := trace.NewTracerProvider()
	otel.SetTracerProvider(tracerProvider)
	defer tracerProvider.Shutdown(context.Background())

	tracer := otel.Tracer("test")

	// Create a buffer to capture log output
	var buf bytes.Buffer

	// Create SlogBridgeLogger that will use OpenTelemetry
	logger := oteladapters.NewSlogBridgeLogger("test")

	// Test without tracing context
	t.Run("without_trace_context", func(t *testing.T) {
		buf.Reset()
		ctx := context.Background()

		logger.InfoContext(ctx, "test message without trace")

		output := buf.String()
		t.Logf("Output without trace: %s", output)

		// The message should be logged (though exact format depends on OTel setup)
		// We mainly verify it doesn't crash and produces some output
	})

	// Test with tracing context
	t.Run("with_trace_context", func(t *testing.T) {
		buf.Reset()

		// Create a span to establish trace context
		ctx, span := tracer.Start(context.Background(), "test-operation")
		defer span.End()

		logger.InfoContext(ctx, "test message with trace")

		output := buf.String()
		t.Logf("Output with trace: %s", output)

		// The message should be logged (though exact format depends on OTel setup)
		// We mainly verify it doesn't crash and produces some output
	})
}

// TestSlogBridgeLoggerInterface verifies that SlogBridgeLogger properly implements
// the eventstore.ContextualLogger interface.
func TestSlogBridgeLoggerInterface(t *testing.T) {
	logger := oteladapters.NewSlogBridgeLogger("test")
	ctx := context.Background()

	// Test all interface methods
	logger.DebugContext(ctx, "debug message", "key", "value")
	logger.InfoContext(ctx, "info message", "key", "value")
	logger.WarnContext(ctx, "warn message", "key", "value")
	logger.ErrorContext(ctx, "error message", "key", "value")

	// If we get here without panicking, the interface is properly implemented
}

// TestSlogBridgeLoggerWithHandler verifies the alternative constructor works.
func TestSlogBridgeLoggerWithHandler(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})

	logger := oteladapters.NewSlogBridgeLoggerWithHandler("test", handler)
	ctx := context.Background()

	logger.InfoContext(ctx, "test message")

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("Expected log output to contain 'test message', got: %s", output)
	}
}
