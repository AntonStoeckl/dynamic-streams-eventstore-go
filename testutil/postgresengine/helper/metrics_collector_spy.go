package helper

import (
	"sync"
	"time"
)

// MetricsCollectorSpy is a MetricsCollector implementation that captures metrics calls for testing.
// It implements the same interface as OpenTelemetry metrics collectors, making it suitable for testing
// EventStore observability instrumentation that follows OpenTelemetry standards.
type MetricsCollectorSpy struct {
	durationRecords []SpyDurationRecord
	counterRecords  []SpyCounterRecord
	valueRecords    []SpyValueRecord
	mu              sync.Mutex
	recordCalls     bool
}

// SpyDurationRecord represents a recorded duration metric call.
type SpyDurationRecord struct {
	Metric   string
	Duration time.Duration
	Labels   map[string]string
}

// SpyCounterRecord represents a recorded counter increment call.
type SpyCounterRecord struct {
	Metric string
	Labels map[string]string
}

// SpyValueRecord represents a recorded value metric call.
type SpyValueRecord struct {
	Metric string
	Value  float64
	Labels map[string]string
}

// NewMetricsCollectorSpy creates a new MetricsCollectorSpy for testing OpenTelemetry-compatible metrics.
// Set recordCalls to true to capture all metrics calls for inspection in tests.
func NewMetricsCollectorSpy(recordCalls bool) *MetricsCollectorSpy {
	return &MetricsCollectorSpy{
		durationRecords: make([]SpyDurationRecord, 0),
		counterRecords:  make([]SpyCounterRecord, 0),
		valueRecords:    make([]SpyValueRecord, 0),
		recordCalls:     recordCalls,
	}
}

// RecordDuration implements the MetricsCollector interface for OpenTelemetry-compatible duration metrics.
func (s *MetricsCollectorSpy) RecordDuration(metric string, duration time.Duration, labels map[string]string) {
	if !s.recordCalls {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Make a copy of labels to avoid external modifications
	labelsCopy := make(map[string]string)
	for k, v := range labels {
		labelsCopy[k] = v
	}

	s.durationRecords = append(s.durationRecords, SpyDurationRecord{
		Metric:   metric,
		Duration: duration,
		Labels:   labelsCopy,
	})
}

// IncrementCounter implements the MetricsCollector interface for OpenTelemetry-compatible counter metrics.
func (s *MetricsCollectorSpy) IncrementCounter(metric string, labels map[string]string) {
	if !s.recordCalls {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Make a copy of labels to avoid external modifications
	labelsCopy := make(map[string]string)
	for k, v := range labels {
		labelsCopy[k] = v
	}

	s.counterRecords = append(s.counterRecords, SpyCounterRecord{
		Metric: metric,
		Labels: labelsCopy,
	})
}

// RecordValue implements the MetricsCollector interface for OpenTelemetry-compatible value/gauge metrics.
func (s *MetricsCollectorSpy) RecordValue(metric string, value float64, labels map[string]string) {
	if !s.recordCalls {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Make a copy of labels to avoid external modifications
	labelsCopy := make(map[string]string)
	for k, v := range labels {
		labelsCopy[k] = v
	}

	s.valueRecords = append(s.valueRecords, SpyValueRecord{
		Metric: metric,
		Value:  value,
		Labels: labelsCopy,
	})
}

// GetDurationRecordCount returns the number of captured duration records.
func (s *MetricsCollectorSpy) GetDurationRecordCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.durationRecords)
}

// GetCounterRecordCount returns the number of captured counter records.
func (s *MetricsCollectorSpy) GetCounterRecordCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.counterRecords)
}

// GetValueRecordCount returns the number of captured value records.
func (s *MetricsCollectorSpy) GetValueRecordCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.valueRecords)
}

// GetDurationRecords returns a copy of all captured duration records.
func (s *MetricsCollectorSpy) GetDurationRecords() []SpyDurationRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	records := make([]SpyDurationRecord, len(s.durationRecords))
	copy(records, s.durationRecords)

	return records
}

// GetCounterRecords returns a copy of all captured counter records.
func (s *MetricsCollectorSpy) GetCounterRecords() []SpyCounterRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	records := make([]SpyCounterRecord, len(s.counterRecords))
	copy(records, s.counterRecords)

	return records
}

// GetValueRecords returns a copy of all captured value records.
func (s *MetricsCollectorSpy) GetValueRecords() []SpyValueRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	records := make([]SpyValueRecord, len(s.valueRecords))
	copy(records, s.valueRecords)

	return records
}

// Reset clears all captured metric records.
func (s *MetricsCollectorSpy) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.durationRecords = s.durationRecords[:0]
	s.counterRecords = s.counterRecords[:0]
	s.valueRecords = s.valueRecords[:0]
}

// HasDurationRecord checks if there's a duration record with the specified metric name.
func (s *MetricsCollectorSpy) HasDurationRecord(metric string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, record := range s.durationRecords {
		if record.Metric == metric {
			return true
		}
	}

	return false
}

// HasCounterRecord checks if there's a counter record with the specified metric name.
func (s *MetricsCollectorSpy) HasCounterRecord(metric string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, record := range s.counterRecords {
		if record.Metric == metric {
			return true
		}
	}

	return false
}

// HasValueRecord checks if there's a value record with the specified metric name.
func (s *MetricsCollectorSpy) HasValueRecord(metric string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, record := range s.valueRecords {
		if record.Metric == metric {
			return true
		}
	}

	return false
}

// MetricRecordMatcher provides a fluent interface for checking metric records.
type MetricRecordMatcher struct {
	collector *MetricsCollectorSpy
	found     bool
	labels    map[string]string
}

// HasDurationRecordForMetric starts a fluent chain to check a duration record.
func (s *MetricsCollectorSpy) HasDurationRecordForMetric(metric string) *MetricRecordMatcher {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, record := range s.durationRecords {
		if record.Metric == metric {
			return &MetricRecordMatcher{
				collector: s,
				found:     true,
				labels:    record.Labels,
			}
		}
	}

	return &MetricRecordMatcher{collector: s, found: false}
}

// HasCounterRecordForMetric starts a fluent chain to check a counter record.
func (s *MetricsCollectorSpy) HasCounterRecordForMetric(metric string) *MetricRecordMatcher {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, record := range s.counterRecords {
		if record.Metric == metric {
			return &MetricRecordMatcher{
				collector: s,
				found:     true,
				labels:    record.Labels,
			}
		}
	}

	return &MetricRecordMatcher{collector: s, found: false}
}

// HasValueRecordForMetric starts a fluent chain to check a value record.
func (s *MetricsCollectorSpy) HasValueRecordForMetric(metric string) *MetricRecordMatcher {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, record := range s.valueRecords {
		if record.Metric == metric {
			return &MetricRecordMatcher{
				collector: s,
				found:     true,
				labels:    record.Labels,
			}
		}
	}

	return &MetricRecordMatcher{collector: s, found: false}
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
func (s *MetricsCollectorSpy) CountDurationRecordsForMetric(metric string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for _, record := range s.durationRecords {
		if record.Metric == metric {
			count++
		}
	}

	return count
}

// CountCounterRecordsForMetric counts how many counter records exist for a specific metric.
func (s *MetricsCollectorSpy) CountCounterRecordsForMetric(metric string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for _, record := range s.counterRecords {
		if record.Metric == metric {
			count++
		}
	}

	return count
}

// CountValueRecordsForMetric counts how many value records exist for a specific metric.
func (s *MetricsCollectorSpy) CountValueRecordsForMetric(metric string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for _, record := range s.valueRecords {
		if record.Metric == metric {
			count++
		}
	}

	return count
}
