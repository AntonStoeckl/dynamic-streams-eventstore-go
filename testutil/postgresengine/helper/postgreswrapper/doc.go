// Package postgreswrapper provides test utilities for abstracting over different PostgreSQL database adapters.
//
// This package enables testing of the event store implementation across multiple database drivers
// (pgx, sql.DB, sqlx.DB) using a common Wrapper interface. The specific adapter type is determined
// by the ADAPTER_TYPE environment variable, allowing the same test suite to run against different
// database implementations.
//
// Key features:
//   - Unified interface for different PostgreSQL adapters
//   - Test and benchmark configuration support
//   - Database cleanup and maintenance utilities
//   - Environment-based adapter selection for CI/CD flexibility
//
// Usage:
//
//	// Create wrapper for testing
//	wrapper := CreateWrapperWithTestConfig(t)
//	defer wrapper.Close()
//
//	// Clean up between tests
//	CleanUp(t, wrapper)
//
//	// Use the event store
//	store := wrapper.GetEventStore()
package postgreswrapper
