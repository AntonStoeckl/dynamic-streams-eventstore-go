package helper

import (
	"sync"
	"time"
)

// TestMetricsCollector is a MetricsCollector implementation that captures metrics calls for testing.
// It implements the same interface as OpenTelemetry metrics collectors, making it suitable for testing
// EventStore observability instrumentation that follows OpenTelemetry standards.
type TestMetricsCollector struct {
	durationRecords []DurationRecord
	counterRecords  []CounterRecord
	valueRecords    []ValueRecord
	mu              sync.Mutex
	recordCalls     bool
}

// DurationRecord represents a recorded duration metric call.
type DurationRecord struct {
	Metric   string
	Duration time.Duration
	Labels   map[string]string
}

// CounterRecord represents a recorded counter-increment call.
type CounterRecord struct {
	Metric string
	Labels map[string]string
}

// ValueRecord represents a recorded value metric call.
type ValueRecord struct {
	Metric string
	Value  float64
	Labels map[string]string
}

// NewTestMetricsCollector creates a new TestMetricsCollector for testing OpenTelemetry-compatible metrics.
// Set recordCalls to true to capture all metrics calls for inspection in tests.
func NewTestMetricsCollector(recordCalls bool) *TestMetricsCollector {
	return &TestMetricsCollector{
		durationRecords: make([]DurationRecord, 0),
		counterRecords:  make([]CounterRecord, 0),
		valueRecords:    make([]ValueRecord, 0),
		recordCalls:     recordCalls,
	}
}

// RecordDuration implements the MetricsCollector interface for OpenTelemetry-compatible duration metrics.
func (c *TestMetricsCollector) RecordDuration(metric string, duration time.Duration, labels map[string]string) {
	if !c.recordCalls {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Make a copy of labels to avoid external modifications
	labelsCopy := make(map[string]string)
	for k, v := range labels {
		labelsCopy[k] = v
	}

	c.durationRecords = append(c.durationRecords, DurationRecord{
		Metric:   metric,
		Duration: duration,
		Labels:   labelsCopy,
	})
}

// IncrementCounter implements the MetricsCollector interface for OpenTelemetry-compatible counter metrics.
func (c *TestMetricsCollector) IncrementCounter(metric string, labels map[string]string) {
	if !c.recordCalls {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Make a copy of labels to avoid external modifications
	labelsCopy := make(map[string]string)
	for k, v := range labels {
		labelsCopy[k] = v
	}

	c.counterRecords = append(c.counterRecords, CounterRecord{
		Metric: metric,
		Labels: labelsCopy,
	})
}

// RecordValue implements the MetricsCollector interface for OpenTelemetry-compatible value/gauge metrics.
func (c *TestMetricsCollector) RecordValue(metric string, value float64, labels map[string]string) {
	if !c.recordCalls {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Make a copy of labels to avoid external modifications
	labelsCopy := make(map[string]string)
	for k, v := range labels {
		labelsCopy[k] = v
	}

	c.valueRecords = append(c.valueRecords, ValueRecord{
		Metric: metric,
		Value:  value,
		Labels: labelsCopy,
	})
}

// GetDurationRecordCount returns the number of captured duration records.
func (c *TestMetricsCollector) GetDurationRecordCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return len(c.durationRecords)
}

// GetCounterRecordCount returns the number of captured counter-records.
func (c *TestMetricsCollector) GetCounterRecordCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return len(c.counterRecords)
}

// GetValueRecordCount returns the number of captured value records.
func (c *TestMetricsCollector) GetValueRecordCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return len(c.valueRecords)
}

// GetDurationRecords returns a copy of all captured duration records.
func (c *TestMetricsCollector) GetDurationRecords() []DurationRecord {
	c.mu.Lock()
	defer c.mu.Unlock()

	records := make([]DurationRecord, len(c.durationRecords))
	copy(records, c.durationRecords)

	return records
}

// GetCounterRecords returns a copy of all captured counter-records.
func (c *TestMetricsCollector) GetCounterRecords() []CounterRecord {
	c.mu.Lock()
	defer c.mu.Unlock()

	records := make([]CounterRecord, len(c.counterRecords))
	copy(records, c.counterRecords)

	return records
}

// GetValueRecords returns a copy of all captured value records.
func (c *TestMetricsCollector) GetValueRecords() []ValueRecord {
	c.mu.Lock()
	defer c.mu.Unlock()

	records := make([]ValueRecord, len(c.valueRecords))
	copy(records, c.valueRecords)

	return records
}

// Reset clears all captured metric records.
func (c *TestMetricsCollector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.durationRecords = c.durationRecords[:0]
	c.counterRecords = c.counterRecords[:0]
	c.valueRecords = c.valueRecords[:0]
}

// HasDurationRecord checks if there's a duration record with the specified metric name.
func (c *TestMetricsCollector) HasDurationRecord(metric string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, record := range c.durationRecords {
		if record.Metric == metric {
			return true
		}
	}

	return false
}

// HasCounterRecord checks if there's a counter-record with the specified metric name.
func (c *TestMetricsCollector) HasCounterRecord(metric string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, record := range c.counterRecords {
		if record.Metric == metric {
			return true
		}
	}

	return false
}

// HasValueRecord checks if there's a value record with the specified metric name.
func (c *TestMetricsCollector) HasValueRecord(metric string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, record := range c.valueRecords {
		if record.Metric == metric {
			return true
		}
	}

	return false
}

// MetricRecordMatcher provides a fluent interface for checking metric records.
type MetricRecordMatcher struct {
	collector *TestMetricsCollector
	found     bool
	labels    map[string]string
}

// HasDurationRecordForMetric starts a fluent chain to check a duration record.
func (c *TestMetricsCollector) HasDurationRecordForMetric(metric string) *MetricRecordMatcher {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, record := range c.durationRecords {
		if record.Metric == metric {
			return &MetricRecordMatcher{
				collector: c,
				found:     true,
				labels:    record.Labels,
			}
		}
	}

	return &MetricRecordMatcher{collector: c, found: false}
}

// HasCounterRecordForMetric starts a fluent chain to check a counter-record.
func (c *TestMetricsCollector) HasCounterRecordForMetric(metric string) *MetricRecordMatcher {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, record := range c.counterRecords {
		if record.Metric == metric {
			return &MetricRecordMatcher{
				collector: c,
				found:     true,
				labels:    record.Labels,
			}
		}
	}

	return &MetricRecordMatcher{collector: c, found: false}
}

// HasValueRecordForMetric starts a fluent chain to check a value record.
func (c *TestMetricsCollector) HasValueRecordForMetric(metric string) *MetricRecordMatcher {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, record := range c.valueRecords {
		if record.Metric == metric {
			return &MetricRecordMatcher{
				collector: c,
				found:     true,
				labels:    record.Labels,
			}
		}
	}

	return &MetricRecordMatcher{collector: c, found: false}
}

// WithOperation checks if the record has the specified operation label.
func (m *MetricRecordMatcher) WithOperation(operation string) *MetricRecordMatcher {
	if !m.found {
		return m
	}

	if value, exists := m.labels["operation"]; !exists || value != operation {
		m.found = false
	}

	return m
}

// WithStatus checks if the record has the specified status label.
func (m *MetricRecordMatcher) WithStatus(status string) *MetricRecordMatcher {
	if !m.found {
		return m
	}

	if value, exists := m.labels["status"]; !exists || value != status {
		m.found = false
	}

	return m
}

// WithErrorType checks if the record has the specified error_type label.
func (m *MetricRecordMatcher) WithErrorType(errorType string) *MetricRecordMatcher {
	if !m.found {
		return m
	}

	if value, exists := m.labels["error_type"]; !exists || value != errorType {
		m.found = false
	}

	return m
}

// WithConflictType checks if the record has the specified conflict_type label.
func (m *MetricRecordMatcher) WithConflictType(conflictType string) *MetricRecordMatcher {
	if !m.found {
		return m
	}

	if value, exists := m.labels["conflict_type"]; !exists || value != conflictType {
		m.found = false
	}

	return m
}

// WithLabel checks if the record has the specified label with the given value.
func (m *MetricRecordMatcher) WithLabel(key, value string) *MetricRecordMatcher {
	if !m.found {
		return m
	}

	if labelValue, exists := m.labels[key]; !exists || labelValue != value {
		m.found = false
	}

	return m
}

// Assert returns true if all conditions in the fluent chain were met.
func (m *MetricRecordMatcher) Assert() bool {
	return m.found
}

// CountDurationRecordsForMetric counts how many duration records exist for a specific metric.
func (c *TestMetricsCollector) CountDurationRecordsForMetric(metric string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := 0
	for _, record := range c.durationRecords {
		if record.Metric == metric {
			count++
		}
	}

	return count
}

// CountCounterRecordsForMetric counts how many counter-records exist for a specific metric.
func (c *TestMetricsCollector) CountCounterRecordsForMetric(metric string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := 0
	for _, record := range c.counterRecords {
		if record.Metric == metric {
			count++
		}
	}

	return count
}

// CountValueRecordsForMetric counts how many value records exist for a specific metric.
func (c *TestMetricsCollector) CountValueRecordsForMetric(metric string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := 0
	for _, record := range c.valueRecords {
		if record.Metric == metric {
			count++
		}
	}

	return count
}
