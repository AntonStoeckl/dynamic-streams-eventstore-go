package helper

import (
	"context"
	"sync"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine"
)

// TestContextualLogger is a ContextualLogger implementation that captures contextual logging calls for testing.
// It implements the same interface as OpenTelemetry loggers, making it suitable for testing
// EventStore observability instrumentation that follows OpenTelemetry standards.
type TestContextualLogger struct {
	debugRecords []ContextualLogRecord
	infoRecords  []ContextualLogRecord
	warnRecords  []ContextualLogRecord
	errorRecords []ContextualLogRecord
	mu           sync.Mutex
	recordCalls  bool
}

// ContextualLogRecord represents a recorded contextual log call.
type ContextualLogRecord struct {
	Level   string
	Message string
	Args    []any
	Context context.Context
}

// NewTestContextualLogger creates a new TestContextualLogger instance.
func NewTestContextualLogger(recordCalls bool) *TestContextualLogger {
	return &TestContextualLogger{
		recordCalls: recordCalls,
	}
}

// DebugContext implements the ContextualLogger interface for testing.
func (l *TestContextualLogger) DebugContext(ctx context.Context, msg string, args ...any) {
	if l.recordCalls {
		l.mu.Lock()
		defer l.mu.Unlock()

		l.debugRecords = append(l.debugRecords, ContextualLogRecord{
			Level:   "debug",
			Message: msg,
			Args:    args,
			Context: ctx,
		})
	}
}

// InfoContext implements the ContextualLogger interface for testing.
func (l *TestContextualLogger) InfoContext(ctx context.Context, msg string, args ...any) {
	if l.recordCalls {
		l.mu.Lock()
		defer l.mu.Unlock()

		l.infoRecords = append(l.infoRecords, ContextualLogRecord{
			Level:   "info",
			Message: msg,
			Args:    args,
			Context: ctx,
		})
	}
}

// WarnContext implements the ContextualLogger interface for testing.
func (l *TestContextualLogger) WarnContext(ctx context.Context, msg string, args ...any) {
	if l.recordCalls {
		l.mu.Lock()
		defer l.mu.Unlock()

		l.warnRecords = append(l.warnRecords, ContextualLogRecord{
			Level:   "warn",
			Message: msg,
			Args:    args,
			Context: ctx,
		})
	}
}

// ErrorContext implements the ContextualLogger interface for testing.
func (l *TestContextualLogger) ErrorContext(ctx context.Context, msg string, args ...any) {
	if l.recordCalls {
		l.mu.Lock()
		defer l.mu.Unlock()

		l.errorRecords = append(l.errorRecords, ContextualLogRecord{
			Level:   "error",
			Message: msg,
			Args:    args,
			Context: ctx,
		})
	}
}

// Reset clears all recorded log calls.
func (l *TestContextualLogger) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.debugRecords = l.debugRecords[:0]
	l.infoRecords = l.infoRecords[:0]
	l.warnRecords = l.warnRecords[:0]
	l.errorRecords = l.errorRecords[:0]
}

// GetDebugRecords returns a copy of all debug log records.
func (l *TestContextualLogger) GetDebugRecords() []ContextualLogRecord {
	l.mu.Lock()
	defer l.mu.Unlock()

	return append([]ContextualLogRecord(nil), l.debugRecords...)
}

// GetInfoRecords returns a copy of all info log records.
func (l *TestContextualLogger) GetInfoRecords() []ContextualLogRecord {
	l.mu.Lock()
	defer l.mu.Unlock()

	return append([]ContextualLogRecord(nil), l.infoRecords...)
}

// GetWarnRecords returns a copy of all warn log records.
func (l *TestContextualLogger) GetWarnRecords() []ContextualLogRecord {
	l.mu.Lock()
	defer l.mu.Unlock()

	return append([]ContextualLogRecord(nil), l.warnRecords...)
}

// GetErrorRecords returns a copy of all error log records.
func (l *TestContextualLogger) GetErrorRecords() []ContextualLogRecord {
	l.mu.Lock()
	defer l.mu.Unlock()

	return append([]ContextualLogRecord(nil), l.errorRecords...)
}

// GetTotalRecordCount returns the total number of log records across all levels.
func (l *TestContextualLogger) GetTotalRecordCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()

	return len(l.debugRecords) + len(l.infoRecords) + len(l.warnRecords) + len(l.errorRecords)
}

// HasDebugLog checks if a debug log with the specified message exists.
func (l *TestContextualLogger) HasDebugLog(message string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, record := range l.debugRecords {
		if record.Message == message {
			return true
		}
	}

	return false
}

// HasInfoLog checks if an info log with the specified message exists.
func (l *TestContextualLogger) HasInfoLog(message string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, record := range l.infoRecords {
		if record.Message == message {
			return true
		}
	}

	return false
}

// HasErrorLog checks if an error log with the specified message exists.
func (l *TestContextualLogger) HasErrorLog(message string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, record := range l.errorRecords {
		if record.Message == message {
			return true
		}
	}

	return false
}

// Compile-time check to ensure TestContextualLogger implements ContextualLogger interface.
var _ postgresengine.ContextualLogger = (*TestContextualLogger)(nil)
