// Package pgtesthelpers provides test utilities for PostgreSQL EventStore testing with multi-adapter support.
//
// This package enables testing across different PostgreSQL drivers (pgx, sql.DB, sqlx.DB) through
// a unified Wrapper interface. Test adapter selection is controlled via the ADAPTER_TYPE environment
// variable, enabling comprehensive testing of all database implementations.
//
// Adapter Types:
//
//	PGXPoolWrapper: wraps pgx.Pool for high-performance connection pooling
//	SQLDBWrapper: wraps database/sql for standard library compatibility
//	SQLXWrapper: wraps sqlx.DB for extended SQL functionality
//
// Utility Functions:
//
//	CreateWrapperWithTestConfig: creates appropriate wrapper based on ADAPTER_TYPE env var
//	CreateWrapperWithBenchmarkConfig: creates wrapper optimized for benchmarking
//	CleanUp: removes all events from database for test isolation
//	GetGreatestOccurredAtTimeFromDB: retrieves latest event timestamp for testing
//	GetLatestBookIDFromDB: retrieves most recent book ID for benchmark continuity
//	CleanUpBookEvents: removes events for specific book ID
//	OptimizeDBWhileBenchmarking: runs PostgreSQL optimization during benchmarks
//
// Environment Variables:
//
//	ADAPTER_TYPE: selects adapter (pgx.pool, sql.db, sqlx.db)
//	TEST_PRIMARY_DSN: PostgreSQL primary instance DSN
//	TEST_REPLICA_DSN: PostgreSQL replica instance DSN
package pgtesthelpers
