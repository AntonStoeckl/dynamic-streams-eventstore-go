package oteladapters

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
)

// TracingCollector implements eventstore.TracingCollector using the OpenTelemetry tracing API.
// It provides seamless integration with OpenTelemetry tracing, creating spans for EventStore
// operations and propagating trace context automatically.
type TracingCollector struct {
	tracer trace.Tracer
}

// NewTracingCollector creates a new OpenTelemetry tracing collector.
// The tracer should be created from your OpenTelemetry TracerProvider.
func NewTracingCollector(tracer trace.Tracer) *TracingCollector {
	return &TracingCollector{tracer: tracer}
}

// StartSpan creates a new OpenTelemetry span with the given name and attributes.
// It returns a new context with the span and a SpanContext wrapper for the span.
func (t *TracingCollector) StartSpan(ctx context.Context, name string, attrs map[string]string) (context.Context, eventstore.SpanContext) {
	// Convert attribute map to OpenTelemetry span options
	spanOptions := make([]trace.SpanStartOption, 0, len(attrs))
	for key, value := range attrs {
		spanOptions = append(spanOptions, trace.WithAttributes(attribute.String(key, value)))
	}

	// Start the span
	spanCtx, span := t.tracer.Start(ctx, name, spanOptions...)

	// Wrap the span in our SpanContext implementation
	return spanCtx, &OTelSpanContext{span: span}
}

// FinishSpan completes an OpenTelemetry span with the given status and additional attributes.
// This method extracts the span from the SpanContext wrapper and finishes it.
func (t *TracingCollector) FinishSpan(spanCtx eventstore.SpanContext, status string, attrs map[string]string) {
	if otelSpanCtx, ok := spanCtx.(*OTelSpanContext); ok {
		// Add final attributes before finishing
		for key, value := range attrs {
			otelSpanCtx.span.SetAttributes(attribute.String(key, value))
		}

		// Set span status based on the status string
		otelSpanCtx.setSpanStatus(status)

		// Finish the span
		otelSpanCtx.span.End()
	}
}

// Ensure TracingCollector implements eventstore.TracingCollector
var _ eventstore.TracingCollector = (*TracingCollector)(nil)

// OTelSpanContext implements eventstore.SpanContext by wrapping an OpenTelemetry span.
// It provides the interface methods while maintaining access to the underlying OTel span.
type OTelSpanContext struct {
	span trace.Span
}

// SetStatus sets the OpenTelemetry span status based on the provided status string.
// It maps common status strings to appropriate OpenTelemetry status codes.
func (s *OTelSpanContext) SetStatus(status string) {
	s.setSpanStatus(status)
}

// AddAttribute adds an attribute to the OpenTelemetry span.
func (s *OTelSpanContext) AddAttribute(key, value string) {
	s.span.SetAttributes(attribute.String(key, value))
}

// setSpanStatus maps status strings to OpenTelemetry span status codes.
// This internal method handles the mapping from our generic status strings
// to OpenTelemetry-specific status codes and descriptions.
func (s *OTelSpanContext) setSpanStatus(status string) {
	switch status {
	case "ok", "success", "completed":
		s.span.SetStatus(codes.Ok, "")
	case "error", "failed", "failure":
		s.span.SetStatus(codes.Error, "Operation failed")
	case "cancelled", "canceled":
		s.span.SetStatus(codes.Error, "Operation cancelled")
	case "timeout":
		s.span.SetStatus(codes.Error, "Operation timed out")
	case "conflict":
		s.span.SetStatus(codes.Error, "Concurrency conflict")
	default:
		// For unknown status strings, record as span attribute
		s.span.SetAttributes(attribute.String("status", status))
	}
}

// Ensure OTelSpanContext implements eventstore.SpanContext
var _ eventstore.SpanContext = (*OTelSpanContext)(nil)
