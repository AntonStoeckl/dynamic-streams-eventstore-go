package helper

import (
	"context"
	"sync"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine"
)

// TestSpanContext implements the SpanContext interface for testing tracing functionality.
type TestSpanContext struct {
	status     string
	attributes map[string]string
	mu         sync.Mutex
}

// SetStatus implements the SpanContext interface for testing.
func (s *TestSpanContext) SetStatus(status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = status
}

// AddAttribute implements the SpanContext interface for testing.
func (s *TestSpanContext) AddAttribute(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.attributes == nil {
		s.attributes = make(map[string]string)
	}
	s.attributes[key] = value
}

// GetStatus returns the current status of the span for testing.
func (s *TestSpanContext) GetStatus() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// GetAttributes returns a copy of all attributes for testing.
func (s *TestSpanContext) GetAttributes() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()

	attrs := make(map[string]string)
	for k, v := range s.attributes {
		attrs[k] = v
	}
	return attrs
}

// TestTracingCollector is a TracingCollector implementation that captures tracing calls for testing.
// It implements the same interface pattern as TestMetricsCollector, making it suitable for testing
// EventStore tracing instrumentation that follows dependency-free tracing standards.
type TestTracingCollector struct {
	spanRecords []SpanRecord
	mu          sync.Mutex
	recordCalls bool
}

// SpanRecord represents a recorded span operation for testing.
type SpanRecord struct {
	Name            string
	StartAttributes map[string]string
	Status          string
	EndAttributes   map[string]string
	SpanContext     *TestSpanContext
}

// NewTestTracingCollector creates a new TestTracingCollector for testing dependency-free tracing.
// Set recordCalls to true to capture all tracing calls for inspection in tests.
func NewTestTracingCollector(recordCalls bool) *TestTracingCollector {
	return &TestTracingCollector{
		spanRecords: make([]SpanRecord, 0),
		recordCalls: recordCalls,
	}
}

// StartSpan implements the TracingCollector interface for testing.
func (c *TestTracingCollector) StartSpan(ctx context.Context, name string, attrs map[string]string) (context.Context, postgresengine.SpanContext) {
	if !c.recordCalls {
		return ctx, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Make a copy of attributes to avoid external modifications
	attrsCopy := make(map[string]string)
	for k, v := range attrs {
		attrsCopy[k] = v
	}

	spanCtx := &TestSpanContext{
		attributes: make(map[string]string),
	}

	record := SpanRecord{
		Name:            name,
		StartAttributes: attrsCopy,
		SpanContext:     spanCtx,
	}

	c.spanRecords = append(c.spanRecords, record)

	return ctx, spanCtx
}

// FinishSpan implements the TracingCollector interface for testing.
func (c *TestTracingCollector) FinishSpan(spanCtx postgresengine.SpanContext, status string, attrs map[string]string) {
	if !c.recordCalls || spanCtx == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	testSpanCtx, ok := spanCtx.(*TestSpanContext)
	if !ok {
		return
	}

	// Make a copy of finish attributes to avoid external modifications
	attrsCopy := make(map[string]string)
	for k, v := range attrs {
		attrsCopy[k] = v
	}

	// Find the corresponding span record and update it
	for i := range c.spanRecords {
		if c.spanRecords[i].SpanContext == testSpanCtx {
			c.spanRecords[i].Status = status
			c.spanRecords[i].EndAttributes = attrsCopy
			break
		}
	}
}

// GetSpanRecordCount returns the number of captured span records.
func (c *TestTracingCollector) GetSpanRecordCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return len(c.spanRecords)
}

// GetSpanRecords returns a copy of all captured span records.
func (c *TestTracingCollector) GetSpanRecords() []SpanRecord {
	c.mu.Lock()
	defer c.mu.Unlock()

	records := make([]SpanRecord, len(c.spanRecords))
	copy(records, c.spanRecords)

	return records
}

// Reset clears all captured span records.
func (c *TestTracingCollector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.spanRecords = c.spanRecords[:0]
}

// HasSpanRecord checks if there's a span record with the specified name.
func (c *TestTracingCollector) HasSpanRecord(name string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, record := range c.spanRecords {
		if record.Name == name {
			return true
		}
	}

	return false
}

// SpanRecordMatcher provides a fluent interface for checking span records.
type SpanRecordMatcher struct {
	collector *TestTracingCollector
	found     bool
	record    *SpanRecord
}

// HasSpanRecordForName starts a fluent chain to check a span record.
func (c *TestTracingCollector) HasSpanRecordForName(name string) *SpanRecordMatcher {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i := range c.spanRecords {
		if c.spanRecords[i].Name == name {
			return &SpanRecordMatcher{
				collector: c,
				found:     true,
				record:    &c.spanRecords[i],
			}
		}
	}

	return &SpanRecordMatcher{collector: c, found: false}
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
func (c *TestTracingCollector) CountSpanRecordsForName(name string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := 0
	for _, record := range c.spanRecords {
		if record.Name == name {
			count++
		}
	}

	return count
}
