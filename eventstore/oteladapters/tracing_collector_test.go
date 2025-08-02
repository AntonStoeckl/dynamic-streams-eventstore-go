package oteladapters_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/oteladapters"
)

func Test_NewTracingCollector_Construction(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := trace.NewTracerProvider(trace.WithSyncer(exporter))
	tracer := provider.Tracer("test")

	collector := oteladapters.NewTracingCollector(tracer)

	assert.NotNil(t, collector, "NewTracingCollector should return non-nil collector")
}

func Test_TracingCollector_StartSpan(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := trace.NewTracerProvider(trace.WithSyncer(exporter))
	tracer := provider.Tracer("test")

	collector := oteladapters.NewTracingCollector(tracer)

	// Start a span with attributes
	attrs := map[string]string{
		"operation": "test_query",
		"database":  "eventstore",
		"table":     "events",
	}

	ctx, spanCtx := collector.StartSpan(context.Background(), "eventstore.query", attrs)

	assert.NotNil(t, ctx, "Context should not be nil")
	assert.NotNil(t, spanCtx, "SpanContext should not be nil")

	// Finish the span to capture it
	collector.FinishSpan(spanCtx, "success", map[string]string{"result": "ok"})

	// Verify span was created correctly
	spans := exporter.GetSpans()
	require.Len(t, spans, 1, "Expected exactly one span")

	span := spans[0]
	assert.Equal(t, "eventstore.query", span.Name, "Span name should match")

	// Verify initial attributes
	assertSpanHasAttribute(t, span, "operation", "test_query")
	assertSpanHasAttribute(t, span, "database", "eventstore")
	assertSpanHasAttribute(t, span, "table", "events")

	// Verify final attributes
	assertSpanHasAttribute(t, span, "result", "ok")

	// Verify span status
	assert.Equal(t, codes.Ok, span.Status.Code, "Span should have OK status")
}

func Test_TracingCollector_FinishSpan_Success(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := trace.NewTracerProvider(trace.WithSyncer(exporter))
	tracer := provider.Tracer("test")

	collector := oteladapters.NewTracingCollector(tracer)

	// Start and finish span with success status
	_, spanCtx := collector.StartSpan(context.Background(), "test-operation", nil)
	collector.FinishSpan(spanCtx, "ok", map[string]string{
		"result":      "completed",
		"event_count": "5",
	})

	spans := exporter.GetSpans()
	require.Len(t, spans, 1, "Expected exactly one span")

	span := spans[0]
	assert.Equal(t, codes.Ok, span.Status.Code, "Span should have OK status")
	assertSpanHasAttribute(t, span, "result", "completed")
	assertSpanHasAttribute(t, span, "event_count", "5")
}

func Test_TracingCollector_FinishSpan_Error(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := trace.NewTracerProvider(trace.WithSyncer(exporter))
	tracer := provider.Tracer("test")

	collector := oteladapters.NewTracingCollector(tracer)

	// Start and finish span with error status
	_, spanCtx := collector.StartSpan(context.Background(), "test-operation", nil)
	collector.FinishSpan(spanCtx, "error", map[string]string{
		"error_type": "database_error",
		"error_code": "connection_failed",
	})

	spans := exporter.GetSpans()
	require.Len(t, spans, 1, "Expected exactly one span")

	span := spans[0]
	assert.Equal(t, codes.Error, span.Status.Code, "Span should have Error status")
	assert.Equal(t, "Operation failed", span.Status.Description, "Error description should match")
	assertSpanHasAttribute(t, span, "error_type", "database_error")
	assertSpanHasAttribute(t, span, "error_code", "connection_failed")
}

func Test_TracingCollector_StatusMapping(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := trace.NewTracerProvider(trace.WithSyncer(exporter))
	tracer := provider.Tracer("test")

	collector := oteladapters.NewTracingCollector(tracer)

	testCases := []struct {
		status              string
		expectedCode        codes.Code
		expectedDescription string
	}{
		{"ok", codes.Ok, ""},
		{"success", codes.Ok, ""},
		{"completed", codes.Ok, ""},
		{"error", codes.Error, "Operation failed"},
		{"failed", codes.Error, "Operation failed"},
		{"failure", codes.Error, "Operation failed"},
		{"cancelled", codes.Error, "Operation cancelled"},
		{"canceled", codes.Error, "Operation cancelled"},
		{"timeout", codes.Error, "Operation timed out"},
		{"conflict", codes.Error, "Concurrency conflict"},
	}

	for _, tc := range testCases {
		t.Run(tc.status, func(t *testing.T) {
			exporter.Reset()

			_, spanCtx := collector.StartSpan(context.Background(), "test", nil)
			collector.FinishSpan(spanCtx, tc.status, nil)

			spans := exporter.GetSpans()
			require.Len(t, spans, 1, "Expected exactly one span")

			span := spans[0]
			assert.Equal(t, tc.expectedCode, span.Status.Code, "Status code should match")
			assert.Equal(t, tc.expectedDescription, span.Status.Description, "Status description should match")
		})
	}
}

func Test_TracingCollector_UnknownStatus(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := trace.NewTracerProvider(trace.WithSyncer(exporter))
	tracer := provider.Tracer("test")

	collector := oteladapters.NewTracingCollector(tracer)

	// Use unknown status string
	_, spanCtx := collector.StartSpan(context.Background(), "test", nil)
	collector.FinishSpan(spanCtx, "unknown_status", nil)

	spans := exporter.GetSpans()
	require.Len(t, spans, 1, "Expected exactly one span")

	span := spans[0]
	// Unknown status should be recorded as an attribute, not span status
	assertSpanHasAttribute(t, span, "status", "unknown_status")
}

func Test_TracingCollector_EmptyAttributes(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := trace.NewTracerProvider(trace.WithSyncer(exporter))
	tracer := provider.Tracer("test")

	collector := oteladapters.NewTracingCollector(tracer)

	// Test with empty attributes
	_, spanCtx := collector.StartSpan(context.Background(), "test-empty", map[string]string{})
	collector.FinishSpan(spanCtx, "ok", map[string]string{})

	// Test with nil attributes
	_, spanCtx2 := collector.StartSpan(context.Background(), "test-nil", nil)
	collector.FinishSpan(spanCtx2, "ok", nil)

	spans := exporter.GetSpans()
	require.Len(t, spans, 2, "Expected exactly two spans")

	// Both should complete successfully without errors
	for _, span := range spans {
		assert.Equal(t, codes.Ok, span.Status.Code, "Spans should complete successfully")
	}
}

func Test_TracingCollector_ContextPropagation(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := trace.NewTracerProvider(trace.WithSyncer(exporter))
	tracer := provider.Tracer("test")

	collector := oteladapters.NewTracingCollector(tracer)

	// Start a parent span manually
	parentCtx, parentSpan := tracer.Start(context.Background(), "parent-operation")
	defer parentSpan.End()

	// Start a child span through the collector
	childCtx, childSpanCtx := collector.StartSpan(parentCtx, "child-operation", nil)
	collector.FinishSpan(childSpanCtx, "ok", nil)

	assert.NotEqual(t, parentCtx, childCtx, "Child context should be different from parent")

	spans := exporter.GetSpans()
	require.Len(t, spans, 1, "Expected exactly one span from collector")

	childSpan := spans[0]
	assert.Equal(t, "child-operation", childSpan.Name, "Child span name should match")

	// The child span should have the parent span as its parent
	assert.Equal(t, parentSpan.SpanContext().SpanID(), childSpan.Parent.SpanID(), "Child should have correct parent")
}

func Test_TracingCollector_NilTracer(t *testing.T) {
	collector := oteladapters.NewTracingCollector(nil)
	assert.NotNil(t, collector, "NewTracingCollector should handle nil tracer")

	// This will panic with nil tracer - this documents the current behavior
	assert.Panics(t, func() {
		collector.StartSpan(context.Background(), "test", nil)
	}, "StartSpan should panic with nil tracer")
}

func Test_TracingCollector_InvalidSpanContext(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := trace.NewTracerProvider(trace.WithSyncer(exporter))
	tracer := provider.Tracer("test")

	collector := oteladapters.NewTracingCollector(tracer)

	// Create a mock SpanContext that's not *OTelSpanContext
	invalidSpanCtx := &mockSpanContext{}

	// FinishSpan should handle invalid SpanContext gracefully
	assert.NotPanics(t, func() {
		collector.FinishSpan(invalidSpanCtx, "ok", map[string]string{"test": "value"})
	}, "FinishSpan should not panic with invalid SpanContext")

	// No spans should be recorded since it's not a valid OTelSpanContext
	spans := exporter.GetSpans()
	assert.Len(t, spans, 0, "No spans should be recorded with invalid SpanContext")
}

func Test_OTelSpanContext_Methods(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := trace.NewTracerProvider(trace.WithSyncer(exporter))
	tracer := provider.Tracer("test")

	collector := oteladapters.NewTracingCollector(tracer)

	// Start a span to get an OTelSpanContext
	_, spanCtx := collector.StartSpan(context.Background(), "test-span", nil)

	// Test SetStatus method
	assert.NotPanics(t, func() {
		spanCtx.SetStatus("success")
	}, "SetStatus should not panic")

	// Test AddAttribute method
	assert.NotPanics(t, func() {
		spanCtx.AddAttribute("test_key", "test_value")
	}, "AddAttribute should not panic")

	// Finish the span
	collector.FinishSpan(spanCtx, "completed", nil)

	// Verify the span was created and has the expected attributes
	spans := exporter.GetSpans()
	require.Len(t, spans, 1, "Expected exactly one span")

	span := spans[0]
	assert.Equal(t, "test-span", span.Name, "Span name should match")
	assert.Equal(t, codes.Ok, span.Status.Code, "Span should have OK status")
	assertSpanHasAttribute(t, span, "test_key", "test_value")
}

// mockSpanContext implements eventstore.SpanContext but is not *OTelSpanContext
type mockSpanContext struct{}

func (m *mockSpanContext) SetStatus(status string)        {}
func (m *mockSpanContext) AddAttribute(key, value string) {}

func assertSpanHasAttribute(t *testing.T, span tracetest.SpanStub, key, expectedValue string) {
	t.Helper()
	found := false
	for _, attr := range span.Attributes {
		if attr.Key == attribute.Key(key) && attr.Value.AsString() == expectedValue {
			found = true
			break
		}
	}
	assert.True(t, found, "Span should have attribute %s=%s", key, expectedValue)
}
