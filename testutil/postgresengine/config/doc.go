// Package config provides PostgreSQL database configuration for EventStore testing.
//
// This package contains factory functions for creating database connections
// using EventStore's supported PostgreSQL adapters (pgx.Pool, sql.DB, sqlx.DB)
// with pre-configured test and benchmark database DSNs.
//
// The configurations support both single-node and primary/replica setups
// for testing EventStore's PostgreSQL implementation under different
// database topologies and adapter types.
package config
