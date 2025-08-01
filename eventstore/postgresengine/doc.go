// Package postgresengine provides a PostgreSQL implementation of the eventstore interface.
//
// This package implements dynamic event streams using PostgreSQL as the storage backend,
// supporting multiple database adapters (pgx, sql.DB, sqlx) with atomic operations
// and concurrency control.
//
// Key features:
//   - Multiple database adapter support (PGX, SQL, SQLX)
//   - Atomic event appending with concurrency conflict detection
//   - Dynamic event stream filtering with JSON predicate support
//   - Configurable table names and structured logging support
//   - OpenTelemetry-compatible metrics for comprehensive observability
//   - Transaction-safe operations with proper resource cleanup
//
// Usage examples:
//
//	// Basic usage
//	db, _ := pgxpool.New(context.Background(), dsn)
//	store, _ := postgresengine.NewEventStoreFromPGXPool(db)
//
//	// With logging and metrics (production observability)
//	store, _ := postgresengine.NewEventStoreFromPGXPool(
//		db,
//		postgresengine.WithTableName("my_events"),
//		postgresengine.WithLogger(logger),
//		postgresengine.WithMetrics(metricsCollector),
//	)
//
//	events, maxSeq, _ := store.Query(ctx, filter)
//	err := store.Append(ctx, filter, maxSeq, newEvent)
package postgresengine
