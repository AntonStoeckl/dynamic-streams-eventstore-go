// Package adapters provide database adapter implementations for the PostgreSQL event store.
//
// This package implements the adapter pattern to support multiple PostgreSQL database libraries:
// pgx.Pool, sql.DB, and sqlx.DB. All adapters provide equivalent functionality through
// a common DBAdapter interface, allowing the event store to work seamlessly with any
// supported database connection type.
//
// The adapters handle the specifics of each database library while presenting a
// unified interface for query execution and result handling.
package adapters
