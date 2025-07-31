// Package config provides database configuration helpers for PostgreSQL connections
// for the example: Book circulation in a public library
//
// This package contains factory functions for creating database connections
// using different PostgreSQL drivers (pgx.Pool, sql.DB, sqlx.DB) with
// pre-configured test and benchmark database DSNs.
//
// This package is part of the shell (infrastructure) layer, providing
// database connection configuration for the event sourcing system.
package config
