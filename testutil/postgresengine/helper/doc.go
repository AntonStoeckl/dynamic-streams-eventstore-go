// Package helper provides testing utilities, log handlers, and metrics collectors for PostgreSQL event store testing.
//
// This package contains shared testing infrastructure including
// - Custom log handlers for capturing and validating log output during tests
// - OpenTelemetry-compatible metrics collectors for testing observability instrumentation
// - Common test utilities used across the PostgreSQL event store test suite
//
// The metrics testing infrastructure follows OpenTelemetry standards, making it suitable
// for testing EventStore observability features that are compatible with modern observability platforms.
package helper
