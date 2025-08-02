package helper

import (
	"context"
	"sync"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine"
)

// SpySpanContext implements the SpySpanContext interface for testing tracing functionality.
type SpySpanContext struct {
	status     string
	attributes map[string]string
	mu         sync.Mutex
}

// SetStatus implements the SpySpanContext interface for testing.
func (c *SpySpanContext) SetStatus(status string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = status
}

// AddAttribute implements the SpySpanContext interface for testing.
func (c *SpySpanContext) AddAttribute(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.attributes == nil {
		c.attributes = make(map[string]string)
	}
	c.attributes[key] = value
}

// GetStatus returns the current status of the span for testing.
func (c *SpySpanContext) GetStatus() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.status
}

// GetAttributes returns a copy of all attributes for testing.
func (c *SpySpanContext) GetAttributes() map[string]string {
	c.mu.Lock()
	defer c.mu.Unlock()

	attrs := make(map[string]string)
	for k, v := range c.attributes {
		attrs[k] = v
	}
	return attrs
}

// TracingCollectorSpy is a TracingCollector implementation that captures tracing calls for testing.
// It implements the same interface pattern as MetricsCollectorSpy, making it suitable for testing
// EventStore tracing instrumentation that follows dependency-free tracing standards.
type TracingCollectorSpy struct {
	spanRecords []SpySpanRecord
	mu          sync.Mutex
	recordCalls bool
}

// SpySpanRecord represents a recorded span operation for testing.
type SpySpanRecord struct {
	Name            string
	StartAttributes map[string]string
	Status          string
	EndAttributes   map[string]string
	SpanContext     *SpySpanContext
}

// NewTracingCollectorSpy creates a new TracingCollectorSpy for testing dependency-free tracing.
// Set recordCalls to true to capture all tracing calls for inspection in tests.
func NewTracingCollectorSpy(recordCalls bool) *TracingCollectorSpy {
	return &TracingCollectorSpy{
		spanRecords: make([]SpySpanRecord, 0),
		recordCalls: recordCalls,
	}
}

// StartSpan implements the TracingCollector interface for testing.
func (s *TracingCollectorSpy) StartSpan(ctx context.Context, name string, attrs map[string]string) (context.Context, postgresengine.SpanContext) {
	if !s.recordCalls {
		return ctx, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Make a copy of attributes to avoid external modifications
	attrsCopy := make(map[string]string)
	for k, v := range attrs {
		attrsCopy[k] = v
	}

	spanCtx := &SpySpanContext{
		attributes: make(map[string]string),
	}

	record := SpySpanRecord{
		Name:            name,
		StartAttributes: attrsCopy,
		SpanContext:     spanCtx,
	}

	s.spanRecords = append(s.spanRecords, record)

	return ctx, spanCtx
}

// FinishSpan implements the TracingCollector interface for testing.
func (s *TracingCollectorSpy) FinishSpan(spanCtx postgresengine.SpanContext, status string, attrs map[string]string) {
	if !s.recordCalls || spanCtx == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	testSpanCtx, ok := spanCtx.(*SpySpanContext)
	if !ok {
		return
	}

	// Make a copy of finish attributes to avoid external modifications
	attrsCopy := make(map[string]string)
	for k, v := range attrs {
		attrsCopy[k] = v
	}

	// Find the corresponding span record and update it
	for i := range s.spanRecords {
		if s.spanRecords[i].SpanContext == testSpanCtx {
			s.spanRecords[i].Status = status
			s.spanRecords[i].EndAttributes = attrsCopy
			break
		}
	}
}

// GetSpanRecordCount returns the number of captured span records.
func (s *TracingCollectorSpy) GetSpanRecordCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.spanRecords)
}

// GetSpanRecords returns a copy of all captured span records.
func (s *TracingCollectorSpy) GetSpanRecords() []SpySpanRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	records := make([]SpySpanRecord, len(s.spanRecords))
	copy(records, s.spanRecords)

	return records
}

// Reset clears all captured span records.
func (s *TracingCollectorSpy) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.spanRecords = s.spanRecords[:0]
}

// HasSpanRecord checks if there's a span record with the specified name.
func (s *TracingCollectorSpy) HasSpanRecord(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, record := range s.spanRecords {
		if record.Name == name {
			return true
		}
	}

	return false
}

// SpanRecordMatcher provides a fluent interface for checking span records.
type SpanRecordMatcher struct {
	collector *TracingCollectorSpy
	found     bool
	record    *SpySpanRecord
}

// HasSpanRecordForName starts a fluent chain to check a span record.
func (s *TracingCollectorSpy) HasSpanRecordForName(name string) *SpanRecordMatcher {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.spanRecords {
		if s.spanRecords[i].Name == name {
			return &SpanRecordMatcher{
				collector: s,
				found:     true,
				record:    &s.spanRecords[i],
			}
		}
	}

	return &SpanRecordMatcher{collector: s, found: false}
}

// WithStatus checks if the span record has the specified status.
func (m *SpanRecordMatcher) WithStatus(status string) *SpanRecordMatcher {
	if !m.found || m.record == nil {
		return m
	}

	if m.record.Status != status {
		m.found = false
	}

	return m
}

// WithStartAttribute checks if the span record has the specified start attribute.
func (m *SpanRecordMatcher) WithStartAttribute(key, value string) *SpanRecordMatcher {
	if !m.found || m.record == nil {
		return m
	}

	if attrValue, exists := m.record.StartAttributes[key]; !exists || attrValue != value {
		m.found = false
	}

	return m
}

// WithEndAttribute checks if the span record has the specified end attribute.
func (m *SpanRecordMatcher) WithEndAttribute(key, value string) *SpanRecordMatcher {
	if !m.found || m.record == nil {
		return m
	}

	if attrValue, exists := m.record.EndAttributes[key]; !exists || attrValue != value {
		m.found = false
	}

	return m
}

// WithSpanAttribute checks if the span context has the specified attribute.
func (m *SpanRecordMatcher) WithSpanAttribute(key, value string) *SpanRecordMatcher {
	if !m.found || m.record == nil || m.record.SpanContext == nil {
		return m
	}

	attrs := m.record.SpanContext.GetAttributes()
	if attrValue, exists := attrs[key]; !exists || attrValue != value {
		m.found = false
	}

	return m
}

// Assert returns true if all conditions in the fluent chain were met.
func (m *SpanRecordMatcher) Assert() bool {
	return m.found
}

// CountSpanRecordsForName counts how many span records exist for a specific name.
func (s *TracingCollectorSpy) CountSpanRecordsForName(name string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for _, record := range s.spanRecords {
		if record.Name == name {
			count++
		}
	}

	return count
}
