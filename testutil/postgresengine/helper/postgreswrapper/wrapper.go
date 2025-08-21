package postgreswrapper

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell/config"
)

// Engine type constants.
const (
	typePGXPool = "pgx.pool"
	typeSQLDB   = "sql.db"
	typeSQLXDB  = "sqlx.db"
)

// Wrapper interface to abstract over different engine types.
type Wrapper interface {
	GetEventStore() *postgresengine.EventStore
	Close()
}

// PGXPoolWrapper wraps pgxpool-based testing.
type PGXPoolWrapper struct {
	pool *pgxpool.Pool
	es   *postgresengine.EventStore
}

// GetEventStore returns the event store instance for the PGX pool wrapper.
func (e *PGXPoolWrapper) GetEventStore() *postgresengine.EventStore {
	return e.es
}

// Close closes the underlying PGX pool connection.
func (e *PGXPoolWrapper) Close() {
	e.pool.Close()
}

// SQLDBWrapper wraps sql.DB-based testing.
type SQLDBWrapper struct {
	db *sql.DB
	es *postgresengine.EventStore
}

// GetEventStore returns the event store instance for the SQL DB wrapper.
func (e *SQLDBWrapper) GetEventStore() *postgresengine.EventStore {
	return e.es
}

// Close closes the underlying SQL DB connection.
func (e *SQLDBWrapper) Close() {
	_ = e.db.Close() // ignore error
}

// SQLXWrapper wraps sqlx.DB-based testing.
type SQLXWrapper struct {
	db *sqlx.DB
	es *postgresengine.EventStore
}

// GetEventStore returns the event store instance for the SQLX wrapper.
func (e *SQLXWrapper) GetEventStore() *postgresengine.EventStore {
	return e.es
}

// Close closes the underlying SQLX DB connection.
func (e *SQLXWrapper) Close() {
	_ = e.db.Close() // ignore error
}

// TryCreateEventStoreWithTableName attempts to create an event store with custom options to test table name validation.
func TryCreateEventStoreWithTableName(t testing.TB, options ...postgresengine.Option) error {
	engineTypeFromEnv := strings.ToLower(os.Getenv("ADAPTER_TYPE"))

	switch engineTypeFromEnv {
	case typePGXPool, "":
		connPool, err := pgxpool.NewWithConfig(context.Background(), config.PostgresPGXPoolSingleConfig())
		assert.NoError(t, err, "error connecting to DB pool in test setup")
		defer connPool.Close()

		_, err = postgresengine.NewEventStoreFromPGXPool(connPool, options...)

		return err

	case typeSQLDB:
		db := config.PostgresSQLDBSingleConfig()
		defer func(db *sql.DB) {
			_ = db.Close() // makes no sense to handle this
		}(db)

		_, err := postgresengine.NewEventStoreFromSQLDB(db, options...)

		return err

	case typeSQLXDB:
		db := config.PostgresSQLXSingleConfig()
		defer func(db *sqlx.DB) {
			_ = db.Close() // makes no sense to handle this
		}(db)

		_, err := postgresengine.NewEventStoreFromSQLX(db, options...)

		return err

	default: // neither one of the known types nor empty
		panic(fmt.Sprintf("unsupported wrapper type from env: %s", engineTypeFromEnv))
	}
}

// CreateWrapperWithTestConfig creates a database wrapper configured for testing with the specified options.
func CreateWrapperWithTestConfig(t testing.TB, options ...postgresengine.Option) Wrapper {
	engineTypeFromEnv := strings.ToLower(os.Getenv("ADAPTER_TYPE"))

	switch engineTypeFromEnv {
	case typePGXPool, "":
		connPool, err := pgxpool.NewWithConfig(context.Background(), config.PostgresPGXPoolSingleConfig())
		assert.NoError(t, err, "error connecting to DB pool in test setup")

		es, err := postgresengine.NewEventStoreFromPGXPool(connPool, options...)
		assert.NoError(t, err, "error creating event store")

		return &PGXPoolWrapper{pool: connPool, es: es}

	case typeSQLDB:
		db := config.PostgresSQLDBSingleConfig()

		es, err := postgresengine.NewEventStoreFromSQLDB(db, options...)
		assert.NoError(t, err, "error creating event store")

		return &SQLDBWrapper{db: db, es: es}

	case typeSQLXDB:
		db := config.PostgresSQLXSingleConfig()

		es, err := postgresengine.NewEventStoreFromSQLX(db, options...)
		assert.NoError(t, err, "error creating event store")

		return &SQLXWrapper{db: db, es: es}

	default: // neither one of the known types nor empty
		panic(fmt.Sprintf("unsupported wrapper type from env: %s", engineTypeFromEnv))
	}
}

// CreateWrapperWithBenchmarkConfig creates a database wrapper configured for benchmarking.
func CreateWrapperWithBenchmarkConfig(t testing.TB, options ...postgresengine.Option) Wrapper {
	engineTypeFromEnv := strings.ToLower(os.Getenv("ADAPTER_TYPE"))

	switch engineTypeFromEnv {
	case typePGXPool, "":
		connPool, err := pgxpool.NewWithConfig(context.Background(), config.PostgresPGXPoolPrimaryConfig())
		assert.NoError(t, err, "error connecting to DB pool in test setup")
		es, err := postgresengine.NewEventStoreFromPGXPool(connPool, options...)
		assert.NoError(t, err, "error creating event store")

		return &PGXPoolWrapper{pool: connPool, es: es}

	case typeSQLDB:
		db := config.PostgresSQLDBPrimaryConfig()
		es, err := postgresengine.NewEventStoreFromSQLDB(db, options...)
		assert.NoError(t, err, "error creating event store")

		return &SQLDBWrapper{db: db, es: es}

	case typeSQLXDB:
		db := config.PostgresSQLXPrimaryConfig()
		es, err := postgresengine.NewEventStoreFromSQLX(db, options...)
		assert.NoError(t, err, "error creating event store")

		return &SQLXWrapper{db: db, es: es}

	default: // neither one of the known types nor empty
		panic(fmt.Sprintf("unsupported wrapper type from env: %s", engineTypeFromEnv))
	}
}

// CleanUp truncates both the events and snapshots tables to prepare for the next test.
// This is needed for tests that use the snapshot functionality.
func CleanUp(t testing.TB, wrapper Wrapper) {
	ctx := context.Background()

	switch e := wrapper.(type) {
	case *PGXPoolWrapper:
		_, err := e.pool.Exec(ctx, "TRUNCATE TABLE events, snapshots RESTART IDENTITY")
		assert.NoError(t, err, "error cleaning up the events and snapshots tables")

	case *SQLDBWrapper:
		_, err := e.db.ExecContext(ctx, "TRUNCATE TABLE events, snapshots RESTART IDENTITY")
		assert.NoError(t, err, "error cleaning up the events and snapshots tables")

	case *SQLXWrapper:
		_, err := e.db.ExecContext(ctx, "TRUNCATE TABLE events, snapshots RESTART IDENTITY")
		assert.NoError(t, err, "error cleaning up the events and snapshots tables")

	default:
		panic(fmt.Sprintf("unsupported wrapper type: %T", e))
	}
}

// GetGreatestOccurredAtTimeFromDB retrieves the maximum occurred_at timestamp from the events table.
func GetGreatestOccurredAtTimeFromDB(t testing.TB, wrapper Wrapper) time.Time {
	var greatestOccurredAtTime time.Time
	var err error

	ctx := context.Background()

	switch e := wrapper.(type) {
	case *PGXPoolWrapper:
		row := e.pool.QueryRow(context.Background(), `select COALESCE(max(occurred_at), '2025-01-01T00:00:00Z'::timestamptz) from events`)
		err = row.Scan(&greatestOccurredAtTime)

	case *SQLDBWrapper:
		row := e.db.QueryRowContext(ctx, `select COALESCE(max(occurred_at), '2025-01-01T00:00:00Z'::timestamptz) from events`)
		err = row.Scan(&greatestOccurredAtTime)

	case *SQLXWrapper:
		row := e.db.QueryRowContext(ctx, `select COALESCE(max(occurred_at), '2025-01-01T00:00:00Z'::timestamptz) from events`)
		err = row.Scan(&greatestOccurredAtTime)

	default:
		panic(fmt.Sprintf("unsupported wrapper type: %T", e))
	}

	assert.NoError(t, err, "error in arranging test data")

	return greatestOccurredAtTime
}

// GetLatestBookIDFromDB retrieves the most recent BookID from the events table payload.
func GetLatestBookIDFromDB(t testing.TB, wrapper Wrapper) uuid.UUID {
	var bookID uuid.UUID
	var err error

	ctx := context.Background()

	switch e := wrapper.(type) {
	case *PGXPoolWrapper:
		row := e.pool.QueryRow(ctx, `select max(payload->>'BookID') from events`)
		err = row.Scan(&bookID)

	case *SQLDBWrapper:
		row := e.db.QueryRowContext(ctx, `select max(payload->>'BookID') from events`)
		err = row.Scan(&bookID)

	case *SQLXWrapper:
		row := e.db.QueryRowContext(ctx, `select max(payload->>'BookID') from events`)
		err = row.Scan(&bookID)

	default:
		panic(fmt.Sprintf("unsupported wrapper type: %T", e))
	}

	assert.NoError(t, err, "error in arranging test data")
	assert.NotEmpty(t, bookID, "error in arranging test data")

	return bookID
}

// GuardThatThereAreEnoughFixtureEventsInStore verifies that the events table contains at least the expected number of events.
func GuardThatThereAreEnoughFixtureEventsInStore(wrapper Wrapper, expectedNumEvents int) {
	var cnt int
	var err error

	ctx := context.Background()

	switch e := wrapper.(type) {
	case *PGXPoolWrapper:
		row := e.pool.QueryRow(ctx, `SELECT count(*) FROM events`)
		err = row.Scan(&cnt)

	case *SQLDBWrapper:
		row := e.db.QueryRowContext(ctx, `SELECT count(*) FROM events`)
		err = row.Scan(&cnt)

	case *SQLXWrapper:
		row := e.db.QueryRowContext(ctx, `SELECT count(*) FROM events`)
		err = row.Scan(&cnt)

	default:
		panic(fmt.Sprintf("unsupported wrapper type: %T", e))
	}

	if err != nil {
		panic(err)
	}

	if cnt < expectedNumEvents {
		panic("not enough fixture events in the DB")
	}
}

// CleanUpBookEvents deletes all events for a specific BookID and returns the number of rows affected.
func CleanUpBookEvents(ctx context.Context, wrapper Wrapper, bookID uuid.UUID) (rowsAffected int64, err error) {
	query := fmt.Sprintf(`DELETE FROM events WHERE payload @> '{"BookID": "%s"}'`, bookID.String()) //nolint:gosec

	switch e := wrapper.(type) {
	case *PGXPoolWrapper:
		cmdTag, execErr := e.pool.Exec(ctx, query)
		if execErr != nil {
			return 0, execErr
		}

		return cmdTag.RowsAffected(), nil

	case *SQLDBWrapper:
		result, execErr := e.db.ExecContext(ctx, query)
		if execErr != nil {
			return 0, execErr
		}

		return result.RowsAffected()

	case *SQLXWrapper:
		result, execErr := e.db.ExecContext(ctx, query)
		if execErr != nil {
			return 0, execErr
		}

		return result.RowsAffected()

	default:
		panic(fmt.Sprintf("unsupported wrapper type: %T", e))
	}
}

// OptimizeDBWhileBenchmarking runs VACUUM ANALYZE on the events table to optimize performance during benchmarking.
func OptimizeDBWhileBenchmarking(ctx context.Context, wrapper Wrapper) error {
	query := `VACUUM ANALYZE EVENTS`

	switch e := wrapper.(type) {
	case *PGXPoolWrapper:
		_, execErr := e.pool.Exec(ctx, query)
		if execErr != nil {
			return execErr
		}

		return nil

	case *SQLDBWrapper:
		_, execErr := e.db.ExecContext(ctx, query)
		if execErr != nil {
			return execErr
		}

		return nil

	case *SQLXWrapper:
		_, execErr := e.db.ExecContext(ctx, query)
		if execErr != nil {
			return execErr
		}

		return nil

	default:
		panic(fmt.Sprintf("unsupported wrapper type: %T", e))
	}
}
