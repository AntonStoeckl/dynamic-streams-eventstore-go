package helper

import (
	"context"
	"log/slog"
	"os"
	"sync"
)

// LogHandlerSpy is a slog.Handler implementation that captures log records for testing.
type LogHandlerSpy struct {
	records     []slog.Record
	mu          sync.Mutex
	logToStdout bool
}

// NewLogHandlerSpy creates a new LogHandlerSpy
// Switchable to log to stdout, which can be useful for debugging tests by seeing the actual log output.
func NewLogHandlerSpy(logToStdOut bool) *LogHandlerSpy {
	return &LogHandlerSpy{
		records:     make([]slog.Record, 0),
		logToStdout: logToStdOut,
	}
}

// Handle implements slog.Handler interface.
func (s *LogHandlerSpy) Handle(ctx context.Context, record slog.Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, record)

	// Optionally also log to stdout for debugging
	if s.logToStdout {
		jsonHandler := slog.NewJSONHandler(os.Stdout, nil)
		_ = jsonHandler.Handle(ctx, record)
	}

	return nil
}

// Enabled implements slog.Handler interface.
func (s *LogHandlerSpy) Enabled(_ context.Context, _ slog.Level) bool {
	return true // Always enabled for testing
}

// WithAttrs implements slog.Handler interface.
func (s *LogHandlerSpy) WithAttrs(_ []slog.Attr) slog.Handler {
	// For testing, we don't need to implement this
	return s
}

// WithGroup implements slog.Handler interface.
func (s *LogHandlerSpy) WithGroup(_ string) slog.Handler {
	// For testing, we don't need to implement this
	return s
}

// GetRecordCount returns the number of captured log records.
func (s *LogHandlerSpy) GetRecordCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.records)
}

// GetRecords returns a copy of all captured log records.
func (s *LogHandlerSpy) GetRecords() []slog.Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	records := make([]slog.Record, len(s.records))
	copy(records, s.records)

	return records
}

// Reset clears all captured log records.
func (s *LogHandlerSpy) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = s.records[:0]
}

// HasDebugLog checks if there's a debug-level log record containing the specified message.
func (s *LogHandlerSpy) HasDebugLog(message string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, record := range s.records {
		if record.Level == slog.LevelDebug && record.Message == message {
			return true
		}
	}

	return false
}

// HasDebugLogWithDurationNS checks if there is a debug-level log record with the specified message
// that contains a duration_ns attribute with a non-negative value.
func (s *LogHandlerSpy) HasDebugLogWithDurationNS(message string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, record := range s.records {
		if record.Level == slog.LevelDebug && record.Message == message {
			hasDurationNS := false
			record.Attrs(func(attr slog.Attr) bool {
				if attr.Key == "duration_ns" && attr.Value.Int64() >= 0 {
					hasDurationNS = true
					return false // Stop iteration
				}

				return true // Continue iteration
			})

			if hasDurationNS {
				return true
			}
		}
	}

	return false
}

// HasDebugLogWithDurationMS checks if there is a debug-level log record with the specified message
// that contains a duration_ms attribute with a non-negative value
// Deprecated: Use HasDebugLogWithMessage(msg).WithDurationMS().Assert() instead.
func (s *LogHandlerSpy) HasDebugLogWithDurationMS(message string) bool {
	return s.HasDebugLogWithMessage(message).WithDurationMS().Assert()
}

// SpyLogRecordMatcher provides a fluent interface for checking log record attributes.
type SpyLogRecordMatcher struct {
	handler *LogHandlerSpy
	record  *slog.Record
	found   bool
}

// HasDebugLogWithMessage starts a fluent chain to check a debug-level log record.
func (s *LogHandlerSpy) HasDebugLogWithMessage(message string) *SpyLogRecordMatcher {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, record := range s.records {
		if record.Level == slog.LevelDebug && record.Message == message {
			return &SpyLogRecordMatcher{
				handler: s,
				record:  &record,
				found:   true,
			}
		}
	}

	return &SpyLogRecordMatcher{handler: s, found: false}
}

// HasInfoLogWithMessage starts a fluent chain to check an info-level log record.
func (s *LogHandlerSpy) HasInfoLogWithMessage(message string) *SpyLogRecordMatcher {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, record := range s.records {
		if record.Level == slog.LevelInfo && record.Message == message {
			return &SpyLogRecordMatcher{
				handler: s,
				record:  &record,
				found:   true,
			}
		}
	}

	return &SpyLogRecordMatcher{handler: s, found: false}
}

// WithDurationMS checks if the log record has a duration_ms attribute with a non-negative value.
func (m *SpyLogRecordMatcher) WithDurationMS() *SpyLogRecordMatcher {
	if !m.found {
		return m
	}

	hasDurationMS := false
	m.record.Attrs(func(attr slog.Attr) bool {
		if attr.Key == "duration_ms" {
			// Handle both Int64 and Float64 values for duration
			switch attr.Value.Kind() {
			case slog.KindInt64:
				if attr.Value.Int64() >= 0 {
					hasDurationMS = true
					return false // Stop iteration
				}

			case slog.KindFloat64:
				if attr.Value.Float64() >= 0 {
					hasDurationMS = true
					return false // Stop iteration
				}

			default:
				// Other types are not supported for duration
			}
		}

		return true // Continue iteration
	})

	if !hasDurationMS {
		m.found = false
	}

	return m
}

// WithEventCount checks if the log record has an event_count attribute with a non-negative value.
func (m *SpyLogRecordMatcher) WithEventCount() *SpyLogRecordMatcher {
	if !m.found {
		return m
	}

	hasEventCount := false
	m.record.Attrs(func(attr slog.Attr) bool {
		if attr.Key == "event_count" && attr.Value.Int64() >= 0 {
			hasEventCount = true
			return false // Stop iteration
		}

		return true // Continue iteration
	})

	if !hasEventCount {
		m.found = false
	}

	return m
}

// WithExpectedEvents checks if the log record has an expected_events attribute with a non-negative value.
func (m *SpyLogRecordMatcher) WithExpectedEvents() *SpyLogRecordMatcher {
	if !m.found {
		return m
	}

	hasExpectedEvents := false
	m.record.Attrs(func(attr slog.Attr) bool {
		if attr.Key == "expected_events" && attr.Value.Int64() >= 0 {
			hasExpectedEvents = true
			return false // Stop iteration
		}

		return true // Continue iteration
	})

	if !hasExpectedEvents {
		m.found = false
	}

	return m
}

// WithRowsAffected checks if the log record has a rows_affected attribute with a non-negative value.
func (m *SpyLogRecordMatcher) WithRowsAffected() *SpyLogRecordMatcher {
	if !m.found {
		return m
	}

	hasRowsAffected := false
	m.record.Attrs(func(attr slog.Attr) bool {
		if attr.Key == "rows_affected" && attr.Value.Int64() >= 0 {
			hasRowsAffected = true
			return false // Stop iteration
		}

		return true // Continue iteration
	})

	if !hasRowsAffected {
		m.found = false
	}

	return m
}

// WithExpectedSequence checks if the log record has an expected_sequence attribute with a non-negative value.
func (m *SpyLogRecordMatcher) WithExpectedSequence() *SpyLogRecordMatcher {
	if !m.found {
		return m
	}

	hasExpectedSequence := false
	m.record.Attrs(func(attr slog.Attr) bool {
		if attr.Key == "expected_sequence" {
			// Handle both Int64 and Uint64 values
			switch attr.Value.Kind() {
			case slog.KindInt64:
				if attr.Value.Int64() >= 0 {
					hasExpectedSequence = true
					return false // Stop iteration
				}

			case slog.KindUint64:
				hasExpectedSequence = true
				return false // Stop iteration

			default:
				// Other types are not supported for sequence numbers
			}
		}

		return true // Continue iteration
	})

	if !hasExpectedSequence {
		m.found = false
	}

	return m
}

// Assert returns true if all conditions in the fluent chain were met.
func (m *SpyLogRecordMatcher) Assert() bool {
	return m.found
}

// HasInfoLogWithDurationMS checks if there is an info-level log record with the specified message
// that contains a duration_ms attribute with a non-negative value
// Deprecated: Use HasInfoLogWithMessage(msg).WithDurationMS().Assert() instead.
func (s *LogHandlerSpy) HasInfoLogWithDurationMS(message string) bool {
	return s.HasInfoLogWithMessage(message).WithDurationMS().Assert()
}
