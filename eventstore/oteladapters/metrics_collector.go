package oteladapters

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
)

// MetricsCollector implements eventstore.MetricsCollector using the OpenTelemetry metrics API.
// It automatically maps the eventstore metrics interface to OpenTelemetry instruments:
//   - RecordDuration -> Histogram (for measuring operation durations)
//   - IncrementCounter -> Counter (for counting operations and errors)
//   - RecordValue -> Gauge (for current values like concurrent operations)
type MetricsCollector struct {
	meter      metric.Meter
	histograms map[string]metric.Float64Histogram
	counters   map[string]metric.Int64Counter
	gauges     map[string]metric.Float64Gauge
}

// NewMetricsCollector creates a new OpenTelemetry metrics collector.
// It uses the provided meter to create instruments on-demand as metrics are recorded.
// The meter should be created from your OpenTelemetry MeterProvider.
func NewMetricsCollector(meter metric.Meter) *MetricsCollector {
	return &MetricsCollector{
		meter:      meter,
		histograms: make(map[string]metric.Float64Histogram),
		counters:   make(map[string]metric.Int64Counter),
		gauges:     make(map[string]metric.Float64Gauge),
	}
}

// RecordDuration records a duration measurement using an OpenTelemetry histogram.
// Histograms are ideal for measuring operation durations as they provide
// percentiles, averages, and distribution information.
func (m *MetricsCollector) RecordDuration(metricName string, duration time.Duration, labels map[string]string) {
	histogram := m.getOrCreateHistogram(metricName)
	if histogram == nil {
		return
	}

	// Convert labels map to OpenTelemetry attributes
	attrs := make([]attribute.KeyValue, 0, len(labels))
	for key, value := range labels {
		attrs = append(attrs, attribute.String(key, value))
	}

	// Record duration in seconds (OpenTelemetry convention)
	// Use context.TODO() for backward compatibility
	histogram.Record(context.TODO(), duration.Seconds(), metric.WithAttributes(attrs...))
}

// RecordDurationContext records a duration measurement with context for trace correlation.
func (m *MetricsCollector) RecordDurationContext(ctx context.Context, metricName string, duration time.Duration, labels map[string]string) {
	histogram := m.getOrCreateHistogram(metricName)
	if histogram == nil {
		return
	}

	// Convert labels map to OpenTelemetry attributes
	attrs := make([]attribute.KeyValue, 0, len(labels))
	for key, value := range labels {
		attrs = append(attrs, attribute.String(key, value))
	}

	// Record duration in seconds (OpenTelemetry convention) with context
	histogram.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}

// IncrementCounter increments a counter using an OpenTelemetry counter.
// Counters are monotonically increasing and ideal for counting operations,
// errors, and other event occurrences.
func (m *MetricsCollector) IncrementCounter(metricName string, labels map[string]string) {
	counter := m.getOrCreateCounter(metricName)
	if counter == nil {
		return
	}

	// Convert labels map to OpenTelemetry attributes
	attrs := make([]attribute.KeyValue, 0, len(labels))
	for key, value := range labels {
		attrs = append(attrs, attribute.String(key, value))
	}

	// Use context.TODO() for backward compatibility
	counter.Add(context.TODO(), 1, metric.WithAttributes(attrs...))
}

// IncrementCounterContext increments a counter with context for trace correlation.
func (m *MetricsCollector) IncrementCounterContext(ctx context.Context, metricName string, labels map[string]string) {
	counter := m.getOrCreateCounter(metricName)
	if counter == nil {
		return
	}

	// Convert labels map to OpenTelemetry attributes
	attrs := make([]attribute.KeyValue, 0, len(labels))
	for key, value := range labels {
		attrs = append(attrs, attribute.String(key, value))
	}

	// Increment counter with context
	counter.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordValue records a float64 value using an OpenTelemetry gauge.
// Gauges represent current values that can go up or down, such as
// the number of concurrent operations or current queue size.
func (m *MetricsCollector) RecordValue(metricName string, value float64, labels map[string]string) {
	gauge := m.getOrCreateGauge(metricName)
	if gauge == nil {
		return
	}

	// Convert labels map to OpenTelemetry attributes
	attrs := make([]attribute.KeyValue, 0, len(labels))
	for key, value := range labels {
		attrs = append(attrs, attribute.String(key, value))
	}

	// Use context.TODO() for backward compatibility
	gauge.Record(context.TODO(), value, metric.WithAttributes(attrs...))
}

// RecordValueContext records a float64 value with context for trace correlation.
func (m *MetricsCollector) RecordValueContext(ctx context.Context, metricName string, value float64, labels map[string]string) {
	gauge := m.getOrCreateGauge(metricName)
	if gauge == nil {
		return
	}

	// Convert labels map to OpenTelemetry attributes
	attrs := make([]attribute.KeyValue, 0, len(labels))
	for key, value := range labels {
		attrs = append(attrs, attribute.String(key, value))
	}

	// Record gauge value with context
	gauge.Record(ctx, value, metric.WithAttributes(attrs...))
}

// getOrCreateHistogram gets an existing histogram or creates a new one for the given metric name.
func (m *MetricsCollector) getOrCreateHistogram(name string) metric.Float64Histogram {
	if histogram, exists := m.histograms[name]; exists {
		return histogram
	}

	histogram, err := m.meter.Float64Histogram(
		name,
		metric.WithDescription("EventStore operation duration"),
		metric.WithUnit("s"), // seconds
	)
	if err != nil {
		// In production, you might want to log this error
		return nil
	}

	m.histograms[name] = histogram
	return histogram
}

// getOrCreateCounter gets an existing counter or creates a new one for the given metric name.
func (m *MetricsCollector) getOrCreateCounter(name string) metric.Int64Counter {
	if counter, exists := m.counters[name]; exists {
		return counter
	}

	counter, err := m.meter.Int64Counter(
		name,
		metric.WithDescription("EventStore operation counter"),
	)
	if err != nil {
		// In production, you might want to log this error
		return nil
	}

	m.counters[name] = counter
	return counter
}

// getOrCreateGauge gets an existing gauge or creates a new one for the given metric name.
func (m *MetricsCollector) getOrCreateGauge(name string) metric.Float64Gauge {
	if gauge, exists := m.gauges[name]; exists {
		return gauge
	}

	gauge, err := m.meter.Float64Gauge(
		name,
		metric.WithDescription("EventStore current value"),
	)
	if err != nil {
		// In production, you might want to log this error
		return nil
	}

	m.gauges[name] = gauge
	return gauge
}

// Ensure MetricsCollector implements eventstore.MetricsCollector.
var _ eventstore.MetricsCollector = (*MetricsCollector)(nil)

// Ensure MetricsCollector implements eventstore.ContextualMetricsCollector.
var _ eventstore.ContextualMetricsCollector = (*MetricsCollector)(nil)
