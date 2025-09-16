package testdoubles

import (
	"context"
	"sync"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine"
)

// ContextualLoggerSpy is a ContextualLogger implementation that captures contextual logging calls for testing.
// It implements the same interface as OpenTelemetry loggers, making it suitable for testing
// EventStore observability instrumentation that follows OpenTelemetry standards.
type ContextualLoggerSpy struct {
	debugRecords []SpyContextualLogRecord
	infoRecords  []SpyContextualLogRecord
	warnRecords  []SpyContextualLogRecord
	errorRecords []SpyContextualLogRecord
	mu           sync.Mutex
	recordCalls  bool
}

// SpyContextualLogRecord represents a recorded contextual log call.
type SpyContextualLogRecord struct {
	Level   string
	Message string
	Args    []any
	Context context.Context
}

// NewContextualLoggerSpy creates a new ContextualLoggerSpy instance.
func NewContextualLoggerSpy(recordCalls bool) *ContextualLoggerSpy {
	return &ContextualLoggerSpy{
		recordCalls: recordCalls,
	}
}

// DebugContext implements the ContextualLogger interface for testing.
func (s *ContextualLoggerSpy) DebugContext(ctx context.Context, msg string, args ...any) {
	if s.recordCalls {
		s.mu.Lock()
		defer s.mu.Unlock()

		s.debugRecords = append(s.debugRecords, SpyContextualLogRecord{
			Level:   "debug",
			Message: msg,
			Args:    args,
			Context: ctx,
		})
	}
}

// InfoContext implements the ContextualLogger interface for testing.
func (s *ContextualLoggerSpy) InfoContext(ctx context.Context, msg string, args ...any) {
	if s.recordCalls {
		s.mu.Lock()
		defer s.mu.Unlock()

		s.infoRecords = append(s.infoRecords, SpyContextualLogRecord{
			Level:   "info",
			Message: msg,
			Args:    args,
			Context: ctx,
		})
	}
}

// WarnContext implements the ContextualLogger interface for testing.
func (s *ContextualLoggerSpy) WarnContext(ctx context.Context, msg string, args ...any) {
	if s.recordCalls {
		s.mu.Lock()
		defer s.mu.Unlock()

		s.warnRecords = append(s.warnRecords, SpyContextualLogRecord{
			Level:   "warn",
			Message: msg,
			Args:    args,
			Context: ctx,
		})
	}
}

// ErrorContext implements the ContextualLogger interface for testing.
func (s *ContextualLoggerSpy) ErrorContext(ctx context.Context, msg string, args ...any) {
	if s.recordCalls {
		s.mu.Lock()
		defer s.mu.Unlock()

		s.errorRecords = append(s.errorRecords, SpyContextualLogRecord{
			Level:   "error",
			Message: msg,
			Args:    args,
			Context: ctx,
		})
	}
}

// Reset clears all recorded log calls.
func (s *ContextualLoggerSpy) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.debugRecords = s.debugRecords[:0]
	s.infoRecords = s.infoRecords[:0]
	s.warnRecords = s.warnRecords[:0]
	s.errorRecords = s.errorRecords[:0]
}

// GetDebugRecords returns a copy of all debug log records.
func (s *ContextualLoggerSpy) GetDebugRecords() []SpyContextualLogRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]SpyContextualLogRecord(nil), s.debugRecords...)
}

// GetInfoRecords returns a copy of all info log records.
func (s *ContextualLoggerSpy) GetInfoRecords() []SpyContextualLogRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]SpyContextualLogRecord(nil), s.infoRecords...)
}

// GetWarnRecords returns a copy of all warn log records.
func (s *ContextualLoggerSpy) GetWarnRecords() []SpyContextualLogRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]SpyContextualLogRecord(nil), s.warnRecords...)
}

// GetErrorRecords returns a copy of all error log records.
func (s *ContextualLoggerSpy) GetErrorRecords() []SpyContextualLogRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]SpyContextualLogRecord(nil), s.errorRecords...)
}

// GetTotalRecordCount returns the total number of log records across all levels.
func (s *ContextualLoggerSpy) GetTotalRecordCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.debugRecords) + len(s.infoRecords) + len(s.warnRecords) + len(s.errorRecords)
}

// HasDebugLog checks if a debug log with the specified message exists.
func (s *ContextualLoggerSpy) HasDebugLog(message string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, record := range s.debugRecords {
		if record.Message == message {
			return true
		}
	}

	return false
}

// HasInfoLog checks if an info log with the specified message exists.
func (s *ContextualLoggerSpy) HasInfoLog(message string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, record := range s.infoRecords {
		if record.Message == message {
			return true
		}
	}

	return false
}

// HasErrorLog checks if an error log with the specified message exists.
func (s *ContextualLoggerSpy) HasErrorLog(message string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, record := range s.errorRecords {
		if record.Message == message {
			return true
		}
	}

	return false
}

// Compile-time check to ensure ContextualLoggerSpy implements ContextualLogger interface.
var _ postgresengine.ContextualLogger = (*ContextualLoggerSpy)(nil)
