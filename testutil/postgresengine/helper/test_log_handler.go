package helper

import (
	"context"
	"log/slog"
	"os"
	"sync"
)

// TestLogHandler is a slog.Handler implementation that captures log records for testing.
type TestLogHandler struct {
	records     []slog.Record
	mu          sync.Mutex
	logToStdout bool
}

// NewTestLogHandler creates a new TestLogHandler
// Switchable to log to stdout, which can be useful for debugging tests by seeing the actual log output.
func NewTestLogHandler(logToStdOut bool) *TestLogHandler {
	return &TestLogHandler{
		records:     make([]slog.Record, 0),
		logToStdout: logToStdOut,
	}
}

// Handle implements slog.Handler interface.
func (h *TestLogHandler) Handle(ctx context.Context, record slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, record)

	// Optionally also log to stdout for debugging
	if h.logToStdout {
		jsonHandler := slog.NewJSONHandler(os.Stdout, nil)
		_ = jsonHandler.Handle(ctx, record)
	}

	return nil
}

// Enabled implements slog.Handler interface.
func (h *TestLogHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true // Always enabled for testing
}

// WithAttrs implements slog.Handler interface.
func (h *TestLogHandler) WithAttrs(_ []slog.Attr) slog.Handler {
	// For testing, we don't need to implement this
	return h
}

// WithGroup implements slog.Handler interface.
func (h *TestLogHandler) WithGroup(_ string) slog.Handler {
	// For testing, we don't need to implement this
	return h
}

// GetRecordCount returns the number of captured log records.
func (h *TestLogHandler) GetRecordCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()

	return len(h.records)
}

// GetRecords returns a copy of all captured log records.
func (h *TestLogHandler) GetRecords() []slog.Record {
	h.mu.Lock()
	defer h.mu.Unlock()
	records := make([]slog.Record, len(h.records))
	copy(records, h.records)

	return records
}

// Reset clears all captured log records.
func (h *TestLogHandler) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = h.records[:0]
}

// HasDebugLog checks if there's a debug-level log record containing the specified message.
func (h *TestLogHandler) HasDebugLog(message string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, record := range h.records {
		if record.Level == slog.LevelDebug && record.Message == message {
			return true
		}
	}

	return false
}

// HasDebugLogWithDurationNS checks if there is a debug-level log record with the specified message
// that contains a duration_ns attribute with a non-negative value.
func (h *TestLogHandler) HasDebugLogWithDurationNS(message string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, record := range h.records {
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
func (h *TestLogHandler) HasDebugLogWithDurationMS(message string) bool {
	return h.HasDebugLogWithMessage(message).WithDurationMS().Assert()
}

// LogRecordMatcher provides a fluent interface for checking log record attributes.
type LogRecordMatcher struct {
	handler *TestLogHandler
	record  *slog.Record
	found   bool
}

// HasDebugLogWithMessage starts a fluent chain to check a debug-level log record.
func (h *TestLogHandler) HasDebugLogWithMessage(message string) *LogRecordMatcher {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, record := range h.records {
		if record.Level == slog.LevelDebug && record.Message == message {
			return &LogRecordMatcher{
				handler: h,
				record:  &record,
				found:   true,
			}
		}
	}

	return &LogRecordMatcher{handler: h, found: false}
}

// HasInfoLogWithMessage starts a fluent chain to check an info-level log record.
func (h *TestLogHandler) HasInfoLogWithMessage(message string) *LogRecordMatcher {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, record := range h.records {
		if record.Level == slog.LevelInfo && record.Message == message {
			return &LogRecordMatcher{
				handler: h,
				record:  &record,
				found:   true,
			}
		}
	}

	return &LogRecordMatcher{handler: h, found: false}
}

// WithDurationMS checks if the log record has a duration_ms attribute with a non-negative value.
func (m *LogRecordMatcher) WithDurationMS() *LogRecordMatcher {
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
func (m *LogRecordMatcher) WithEventCount() *LogRecordMatcher {
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
func (m *LogRecordMatcher) WithExpectedEvents() *LogRecordMatcher {
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
func (m *LogRecordMatcher) WithRowsAffected() *LogRecordMatcher {
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
func (m *LogRecordMatcher) WithExpectedSequence() *LogRecordMatcher {
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
func (m *LogRecordMatcher) Assert() bool {
	return m.found
}

// HasInfoLogWithDurationMS checks if there is an info-level log record with the specified message
// that contains a duration_ms attribute with a non-negative value
// Deprecated: Use HasInfoLogWithMessage(msg).WithDurationMS().Assert() instead.
func (h *TestLogHandler) HasInfoLogWithDurationMS(message string) bool {
	return h.HasInfoLogWithMessage(message).WithDurationMS().Assert()
}
