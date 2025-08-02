package oteladapters_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/oteladapters"
)

func Test_NewMetricsCollector_Construction(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("test")

	collector := oteladapters.NewMetricsCollector(meter)

	assert.NotNil(t, collector, "NewMetricsCollector should return non-nil collector")
}

func Test_MetricsCollector_RecordDuration(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("test")

	collector := oteladapters.NewMetricsCollector(meter)

	// Record a duration metric
	testDuration := 150 * time.Millisecond
	labels := map[string]string{
		"operation": "test_query",
		"status":    "success",
	}

	collector.RecordDuration("eventstore_query_duration_seconds", testDuration, labels)

	// Collect metrics and verify
	var resourceMetrics metricdata.ResourceMetrics
	err := reader.Collect(context.Background(), &resourceMetrics)
	require.NoError(t, err, "Failed to collect metrics")

	// Find our histogram
	histogram := findHistogramMetric(t, resourceMetrics, "eventstore_query_duration_seconds")
	require.Len(t, histogram.DataPoints, 1, "Expected exactly one data point")

	dataPoint := histogram.DataPoints[0]

	// Verify the recorded value (150 ms = 0.15 seconds)
	assert.Equal(t, uint64(1), dataPoint.Count, "Histogram count should be 1")
	assert.InDelta(t, 0.15, dataPoint.Sum, 0.001, "Histogram sum should be 0.15 seconds")

	// Verify attributes
	expectedAttrs := attribute.NewSet(
		attribute.String("operation", "test_query"),
		attribute.String("status", "success"),
	)
	assert.True(t, dataPoint.Attributes.Equals(&expectedAttrs), "Attributes should match")
}

func Test_MetricsCollector_IncrementCounter(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("test")

	collector := oteladapters.NewMetricsCollector(meter)

	// Increment counter multiple times
	labels := map[string]string{
		"operation":   "append",
		"status":      "success",
		"result_type": "created",
	}

	collector.IncrementCounter("eventstore_operations_total", labels)
	collector.IncrementCounter("eventstore_operations_total", labels)
	collector.IncrementCounter("eventstore_operations_total", labels)

	// Collect metrics and verify
	var resourceMetrics metricdata.ResourceMetrics
	err := reader.Collect(context.Background(), &resourceMetrics)
	require.NoError(t, err, "Failed to collect metrics")

	// Find our counter
	counter := findCounterMetric(t, resourceMetrics, "eventstore_operations_total")
	require.Len(t, counter.DataPoints, 1, "Expected exactly one data point")

	dataPoint := counter.DataPoints[0]

	// Verify the incremented value
	assert.Equal(t, int64(3), dataPoint.Value, "Counter should have been incremented 3 times")

	// Verify attributes
	expectedAttrs := attribute.NewSet(
		attribute.String("operation", "append"),
		attribute.String("status", "success"),
		attribute.String("result_type", "created"),
	)
	assert.True(t, dataPoint.Attributes.Equals(&expectedAttrs), "Attributes should match")
}

func Test_MetricsCollector_RecordValue(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("test")

	collector := oteladapters.NewMetricsCollector(meter)

	// Record gauge values
	labels := map[string]string{
		"operation": "query",
		"status":    "success",
	}

	collector.RecordValue("eventstore_events_queried_total", 42.5, labels)

	// Collect metrics and verify
	var resourceMetrics metricdata.ResourceMetrics
	err := reader.Collect(context.Background(), &resourceMetrics)
	require.NoError(t, err, "Failed to collect metrics")

	// Find our gauge
	gauge := findGaugeMetric(t, resourceMetrics, "eventstore_events_queried_total")
	require.Len(t, gauge.DataPoints, 1, "Expected exactly one data point")

	dataPoint := gauge.DataPoints[0]

	// Verify the recorded value
	assert.Equal(t, 42.5, dataPoint.Value, "Gauge value should be 42.5")

	// Verify attributes
	expectedAttrs := attribute.NewSet(
		attribute.String("operation", "query"),
		attribute.String("status", "success"),
	)
	assert.True(t, dataPoint.Attributes.Equals(&expectedAttrs), "Attributes should match")
}

func Test_MetricsCollector_ContextualMethods(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("test")

	collector := oteladapters.NewMetricsCollector(meter)

	// Use context-aware methods
	ctx := context.Background()
	labels := map[string]string{"test": "contextual"}

	collector.RecordDurationContext(ctx, "test_duration", 100*time.Millisecond, labels)
	collector.IncrementCounterContext(ctx, "test_counter", labels)
	collector.RecordValueContext(ctx, "test_gauge", 123.45, labels)

	// Collect and verify all metrics were recorded
	var resourceMetrics metricdata.ResourceMetrics
	err := reader.Collect(context.Background(), &resourceMetrics)
	require.NoError(t, err, "Failed to collect metrics")

	metricNames := make(map[string]bool)
	for _, scopeMetrics := range resourceMetrics.ScopeMetrics {
		for _, metric := range scopeMetrics.Metrics {
			metricNames[metric.Name] = true
		}
	}

	assert.True(t, metricNames["test_duration"], "Duration metric should be recorded")
	assert.True(t, metricNames["test_counter"], "Counter metric should be recorded")
	assert.True(t, metricNames["test_gauge"], "Gauge metric should be recorded")
}

func Test_MetricsCollector_EmptyLabels(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("test")

	collector := oteladapters.NewMetricsCollector(meter)

	// Record with empty labels
	collector.RecordDuration("test_metric", 50*time.Millisecond, map[string]string{})

	// Collect and verify
	var resourceMetrics metricdata.ResourceMetrics
	err := reader.Collect(context.Background(), &resourceMetrics)
	require.NoError(t, err, "Failed to collect metrics")

	// Should still record the metric, just with no attributes
	found := false
	for _, scopeMetrics := range resourceMetrics.ScopeMetrics {
		for _, metric := range scopeMetrics.Metrics {
			if metric.Name == "test_metric" {
				found = true
				break
			}
		}
	}

	assert.True(t, found, "Metric should be recorded even with empty labels")
}

func Test_MetricsCollector_NilLabels(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("test")

	collector := oteladapters.NewMetricsCollector(meter)

	// Record with nil labels (should not crash)
	collector.RecordDuration("test_metric", 50*time.Millisecond, nil)

	// Collect and verify
	var resourceMetrics metricdata.ResourceMetrics
	err := reader.Collect(context.Background(), &resourceMetrics)
	require.NoError(t, err, "Failed to collect metrics")

	// Should still record the metric
	found := false
	for _, scopeMetrics := range resourceMetrics.ScopeMetrics {
		for _, metric := range scopeMetrics.Metrics {
			if metric.Name == "test_metric" {
				found = true
				break
			}
		}
	}

	assert.True(t, found, "Metric should be recorded even with nil labels")
}

func Test_MetricsCollector_NilMeter(t *testing.T) {
	collector := oteladapters.NewMetricsCollector(nil)
	assert.NotNil(t, collector, "NewMetricsCollector should handle nil meter")

	// These will panic with nil meter - this documents the current behavior
	assert.Panics(t, func() {
		collector.RecordDuration("test", 100*time.Millisecond, nil)
	}, "RecordDuration should panic with nil meter")

	assert.Panics(t, func() {
		collector.IncrementCounter("test", nil)
	}, "IncrementCounter should panic with nil meter")

	assert.Panics(t, func() {
		collector.RecordValue("test", 42.0, nil)
	}, "RecordValue should panic with nil meter")
}

func Test_MetricsCollector_InstrumentReuse(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("test")

	collector := oteladapters.NewMetricsCollector(meter)

	// Test histogram reuse - record same metric multiple times
	collector.RecordDuration("reused_histogram", 100*time.Millisecond, nil)
	collector.RecordDuration("reused_histogram", 200*time.Millisecond, nil)

	// Test counter reuse - increment same counter multiple times
	collector.IncrementCounter("reused_counter", nil)
	collector.IncrementCounter("reused_counter", nil)
	collector.IncrementCounter("reused_counter", nil)

	// Test gauge reuse - record same gauge multiple times (last value wins)
	collector.RecordValue("reused_gauge", 10.0, nil)
	collector.RecordValue("reused_gauge", 20.0, nil)

	// Collect metrics
	var resourceMetrics metricdata.ResourceMetrics
	err := reader.Collect(context.Background(), &resourceMetrics)
	require.NoError(t, err, "Failed to collect metrics")

	// Verify histogram reuse worked - should have aggregated values
	histogram := findHistogramMetric(t, resourceMetrics, "reused_histogram")
	assert.Equal(t, uint64(2), histogram.DataPoints[0].Count, "Should have recorded two durations")

	// Verify counter reuse worked - should have aggregated values
	counter := findCounterMetric(t, resourceMetrics, "reused_counter")
	assert.Equal(t, int64(3), counter.DataPoints[0].Value, "Should have incremented counter 3 times")

	// Verify gauge reuse worked - should have last value
	gauge := findGaugeMetric(t, resourceMetrics, "reused_gauge")
	assert.Equal(t, 20.0, gauge.DataPoints[0].Value, "Should have the last recorded gauge value")
}

func Test_MetricsCollector_InstrumentCreationErrors(t *testing.T) {
	// Create a wrapper meter that causes instrument creation to fail
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	baseMeter := provider.Meter("test")

	errorMeter := &errorInjectingMeter{Meter: baseMeter}
	collector := oteladapters.NewMetricsCollector(errorMeter)

	// These should not panic when instrument creation fails
	assert.NotPanics(t, func() {
		collector.RecordDuration("error_histogram", 100*time.Millisecond, nil)
	}, "RecordDuration should not panic when histogram creation fails")

	assert.NotPanics(t, func() {
		collector.IncrementCounter("error_counter", nil)
	}, "IncrementCounter should not panic when counter creation fails")

	assert.NotPanics(t, func() {
		collector.RecordValue("error_gauge", 42.0, nil)
	}, "RecordValue should not panic when gauge creation fails")

	// Context versions should also not panic
	ctx := context.Background()
	assert.NotPanics(t, func() {
		collector.RecordDurationContext(ctx, "error_histogram_ctx", 100*time.Millisecond, nil)
	}, "RecordDurationContext should not panic when histogram creation fails")

	assert.NotPanics(t, func() {
		collector.IncrementCounterContext(ctx, "error_counter_ctx", nil)
	}, "IncrementCounterContext should not panic when counter creation fails")

	assert.NotPanics(t, func() {
		collector.RecordValueContext(ctx, "error_gauge_ctx", 42.0, nil)
	}, "RecordValueContext should not panic when gauge creation fails")
}

// errorInjectingMeter wraps a real meter but returns errors for instruments with "error_" prefix
type errorInjectingMeter struct {
	metric.Meter // Embed the interface to get all methods including unexported ones
}

func (m *errorInjectingMeter) Float64Histogram(name string, options ...metric.Float64HistogramOption) (metric.Float64Histogram, error) {
	if name == "error_histogram" || name == "error_histogram_ctx" {
		return nil, errors.New("histogram creation failed")
	}
	return m.Meter.Float64Histogram(name, options...)
}

func (m *errorInjectingMeter) Int64Counter(name string, options ...metric.Int64CounterOption) (metric.Int64Counter, error) {
	if name == "error_counter" || name == "error_counter_ctx" {
		return nil, errors.New("counter creation failed")
	}
	return m.Meter.Int64Counter(name, options...)
}

func (m *errorInjectingMeter) Float64Gauge(name string, options ...metric.Float64GaugeOption) (metric.Float64Gauge, error) {
	if name == "error_gauge" || name == "error_gauge_ctx" {
		return nil, errors.New("gauge creation failed")
	}
	return m.Meter.Float64Gauge(name, options...)
}

func findHistogramMetric(t *testing.T, resourceMetrics metricdata.ResourceMetrics, name string) *metricdata.Histogram[float64] {
	t.Helper()
	for _, scopeMetrics := range resourceMetrics.ScopeMetrics {
		for _, metric := range scopeMetrics.Metrics {
			if metric.Name == name {
				if h, ok := metric.Data.(metricdata.Histogram[float64]); ok {
					return &h
				}
			}
		}
	}
	t.Fatalf("Histogram metric %s not found", name)
	return nil // This will never be reached
}

func findCounterMetric(t *testing.T, resourceMetrics metricdata.ResourceMetrics, name string) *metricdata.Sum[int64] {
	t.Helper()
	for _, scopeMetrics := range resourceMetrics.ScopeMetrics {
		for _, metric := range scopeMetrics.Metrics {
			if metric.Name == name {
				if c, ok := metric.Data.(metricdata.Sum[int64]); ok {
					return &c
				}
			}
		}
	}
	t.Fatalf("Counter metric %s not found", name)
	return nil // This will never be reached
}

func findGaugeMetric(t *testing.T, resourceMetrics metricdata.ResourceMetrics, name string) *metricdata.Gauge[float64] {
	t.Helper()
	for _, scopeMetrics := range resourceMetrics.ScopeMetrics {
		for _, metric := range scopeMetrics.Metrics {
			if metric.Name == name {
				if g, ok := metric.Data.(metricdata.Gauge[float64]); ok {
					return &g
				}
			}
		}
	}
	t.Fatalf("Gauge metric %s not found", name)
	return nil // This will never be reached
}
