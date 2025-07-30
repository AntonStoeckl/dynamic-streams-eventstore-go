package helper

import (
	"context"
	"log/slog"
	"sync"
)

// TestLogHandler is a slog.Handler implementation that captures log records for testing
type TestLogHandler struct {
	records []slog.Record
	mu      sync.Mutex
}

// NewTestLogHandler creates a new TestLogHandler
func NewTestLogHandler() *TestLogHandler {
	return &TestLogHandler{
		records: make([]slog.Record, 0),
	}
}

// Handle implements slog.Handler interface
func (h *TestLogHandler) Handle(_ context.Context, record slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, record)

	return nil
}

// Enabled implements slog.Handler interface
func (h *TestLogHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true // Always enabled for testing
}

// WithAttrs implements slog.Handler interface
func (h *TestLogHandler) WithAttrs(_ []slog.Attr) slog.Handler {
	// For testing, we don't need to implement this
	return h
}

// WithGroup implements slog.Handler interface
func (h *TestLogHandler) WithGroup(_ string) slog.Handler {
	// For testing, we don't need to implement this
	return h
}

// GetRecordCount returns the number of captured log records
func (h *TestLogHandler) GetRecordCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()

	return len(h.records)
}

// GetRecords returns a copy of all captured log records
func (h *TestLogHandler) GetRecords() []slog.Record {
	h.mu.Lock()
	defer h.mu.Unlock()
	records := make([]slog.Record, len(h.records))
	copy(records, h.records)

	return records
}

// Reset clears all captured log records
func (h *TestLogHandler) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = h.records[:0]
}

// HasDebugLogWithMessage checks if there's a debug-level log record containing the specified message
func (h *TestLogHandler) HasDebugLogWithMessage(message string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, record := range h.records {
		if record.Level == slog.LevelDebug && record.Message == message {
			return true
		}
	}

	return false
}
