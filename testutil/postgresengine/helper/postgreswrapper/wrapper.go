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

// Engine type constants
const (
	typePGXPool = "pgx.pool"
	typeSQLDB   = "sql.db"
	typeSQLXDB  = "sqlx.db"
)

// Wrapper interface to abstract over different engine types
type Wrapper interface {
	GetEventStore() postgresengine.EventStore
	Close()
}

// PGXPoolWrapper wraps pgxpool-based testing
type PGXPoolWrapper struct {
	pool *pgxpool.Pool
	es   postgresengine.EventStore
}

func (e *PGXPoolWrapper) GetEventStore() postgresengine.EventStore {
	return e.es
}

func (e *PGXPoolWrapper) Close() {
	e.pool.Close()
}

// SQLDBWrapper wraps sql.DB-based testing
type SQLDBWrapper struct {
	db *sql.DB
	es postgresengine.EventStore
}

func (e *SQLDBWrapper) GetEventStore() postgresengine.EventStore {
	return e.es
}

func (e *SQLDBWrapper) Close() {
	_ = e.db.Close() // ignore error
}

// SQLXWrapper wraps sqlx.DB-based testing
type SQLXWrapper struct {
	db *sqlx.DB
	es postgresengine.EventStore
}

func (e *SQLXWrapper) GetEventStore() postgresengine.EventStore {
	return e.es
}

func (e *SQLXWrapper) Close() {
	_ = e.db.Close() // ignore error
}

func TryCreateEventStoreWithTableName(t testing.TB, options ...postgresengine.Option) error {
	engineTypeFromEnv := strings.ToLower(os.Getenv("ADAPTER_TYPE"))

	switch engineTypeFromEnv {
	case typePGXPool, "":
		connPool, err := pgxpool.NewWithConfig(context.Background(), config.PostgresPGXPoolTestConfig())
		assert.NoError(t, err, "error connecting to DB pool in test setup")
		defer connPool.Close()

		_, err = postgresengine.NewEventStoreFromPGXPool(connPool, options...)

		return err

	case typeSQLDB:
		db := config.PostgresSQLDBTestConfig()
		defer func(db *sql.DB) {
			_ = db.Close() // makes no sense to handle this
		}(db)

		_, err := postgresengine.NewEventStoreFromSQLDB(db, options...)

		return err

	case typeSQLXDB:
		db := config.PostgresSQLXTestConfig()
		defer func(db *sqlx.DB) {
			_ = db.Close() // makes no sense to handle this
		}(db)

		_, err := postgresengine.NewEventStoreFromSQLX(db, options...)

		return err

	default: // neither one of the known types nor empty
		panic(fmt.Sprintf("unsupported wrapper type from env: %s", engineTypeFromEnv))
	}
}

func CreateWrapperWithTestConfig(t testing.TB, options ...postgresengine.Option) Wrapper {
	engineTypeFromEnv := strings.ToLower(os.Getenv("ADAPTER_TYPE"))

	switch engineTypeFromEnv {
	case typePGXPool, "":
		connPool, err := pgxpool.NewWithConfig(context.Background(), config.PostgresPGXPoolTestConfig())
		assert.NoError(t, err, "error connecting to DB pool in test setup")

		es, err := postgresengine.NewEventStoreFromPGXPool(connPool, options...)
		assert.NoError(t, err, "error creating event store")

		return &PGXPoolWrapper{pool: connPool, es: es}

	case typeSQLDB:
		db := config.PostgresSQLDBTestConfig()

		es, err := postgresengine.NewEventStoreFromSQLDB(db, options...)
		assert.NoError(t, err, "error creating event store")

		return &SQLDBWrapper{db: db, es: es}

	case typeSQLXDB:
		db := config.PostgresSQLXTestConfig()

		es, err := postgresengine.NewEventStoreFromSQLX(db, options...)
		assert.NoError(t, err, "error creating event store")

		return &SQLXWrapper{db: db, es: es}

	default: // neither one of the known types nor empty
		panic(fmt.Sprintf("unsupported wrapper type from env: %s", engineTypeFromEnv))
	}
}

func CreateWrapperWithBenchmarkConfig(t testing.TB) Wrapper {
	engineTypeFromEnv := strings.ToLower(os.Getenv("ADAPTER_TYPE"))

	switch engineTypeFromEnv {
	case typePGXPool, "":
		connPool, err := pgxpool.NewWithConfig(context.Background(), config.PostgresPGXPoolBenchmarkConfig())
		assert.NoError(t, err, "error connecting to DB pool in test setup")
		es, err := postgresengine.NewEventStoreFromPGXPool(connPool)
		assert.NoError(t, err, "error creating event store")

		return &PGXPoolWrapper{pool: connPool, es: es}

	case typeSQLDB:
		db := config.PostgresSQLDBBenchmarkConfig()
		es, err := postgresengine.NewEventStoreFromSQLDB(db)
		assert.NoError(t, err, "error creating event store")

		return &SQLDBWrapper{db: db, es: es}

	case typeSQLXDB:
		db := config.PostgresSQLXBenchmarkConfig()
		es, err := postgresengine.NewEventStoreFromSQLX(db)
		assert.NoError(t, err, "error creating event store")

		return &SQLXWrapper{db: db, es: es}

	default: // neither one of the known types nor empty
		panic(fmt.Sprintf("unsupported wrapper type from env: %s", engineTypeFromEnv))
	}
}

func CleanUp(t testing.TB, wrapper Wrapper) {
	switch e := wrapper.(type) {
	case *PGXPoolWrapper:
		_, err := e.pool.Exec(context.Background(), "TRUNCATE TABLE events RESTART IDENTITY")
		assert.NoError(t, err, "error cleaning up the events table")

	case *SQLDBWrapper:
		_, err := e.db.Exec("TRUNCATE TABLE events RESTART IDENTITY")
		assert.NoError(t, err, "error cleaning up the events table")

	case *SQLXWrapper:
		_, err := e.db.Exec("TRUNCATE TABLE events RESTART IDENTITY")
		assert.NoError(t, err, "error cleaning up the events table")

	default:
		panic(fmt.Sprintf("unsupported wrapper type: %T", e))
	}
}

func GetGreatestOccurredAtTimeFromDB(t testing.TB, wrapper Wrapper) time.Time {
	var greatestOccurredAtTime time.Time
	var err error

	switch e := wrapper.(type) {
	case *PGXPoolWrapper:
		row := e.pool.QueryRow(context.Background(), `select max(occurred_at) from events`)
		err = row.Scan(&greatestOccurredAtTime)

	case *SQLDBWrapper:
		row := e.db.QueryRow(`select max(occurred_at) from events`)
		err = row.Scan(&greatestOccurredAtTime)

	case *SQLXWrapper:
		row := e.db.QueryRow(`select max(occurred_at) from events`)
		err = row.Scan(&greatestOccurredAtTime)

	default:
		panic(fmt.Sprintf("unsupported wrapper type: %T", e))
	}

	assert.NoError(t, err, "error in arranging test data")

	return greatestOccurredAtTime
}

func GetLatestBookIDFromDB(t testing.TB, wrapper Wrapper) uuid.UUID {
	var bookID uuid.UUID
	var err error

	switch e := wrapper.(type) {
	case *PGXPoolWrapper:
		row := e.pool.QueryRow(context.Background(), `select max(payload->>'BookID') from events`)
		err = row.Scan(&bookID)

	case *SQLDBWrapper:
		row := e.db.QueryRow(`select max(payload->>'BookID') from events`)
		err = row.Scan(&bookID)

	case *SQLXWrapper:
		row := e.db.QueryRow(`select max(payload->>'BookID') from events`)
		err = row.Scan(&bookID)

	default:
		panic(fmt.Sprintf("unsupported wrapper type: %T", e))
	}

	assert.NoError(t, err, "error in arranging test data")
	assert.NotEmpty(t, bookID, "error in arranging test data")

	return bookID
}

func GuardThatThereAreEnoughFixtureEventsInStore(wrapper Wrapper, expectedNumEvents int) {
	var cnt int
	var err error

	switch e := wrapper.(type) {
	case *PGXPoolWrapper:
		row := e.pool.QueryRow(context.Background(), `SELECT count(*) FROM events`)
		err = row.Scan(&cnt)

	case *SQLDBWrapper:
		row := e.db.QueryRow(`SELECT count(*) FROM events`)
		err = row.Scan(&cnt)

	case *SQLXWrapper:
		row := e.db.QueryRow(`SELECT count(*) FROM events`)
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

func CleanUpBookEvents(wrapper Wrapper, bookID uuid.UUID) (rowsAffected int64, err error) {
	query := fmt.Sprintf(`DELETE FROM events WHERE payload @> '{"BookID": "%s"}'`, bookID.String())

	switch e := wrapper.(type) {
	case *PGXPoolWrapper:
		cmdTag, execErr := e.pool.Exec(context.Background(), query)
		if execErr != nil {
			return 0, execErr
		}

		return cmdTag.RowsAffected(), nil

	case *SQLDBWrapper:
		result, execErr := e.db.Exec(query)
		if execErr != nil {
			return 0, execErr
		}

		return result.RowsAffected()

	case *SQLXWrapper:
		result, execErr := e.db.Exec(query)
		if execErr != nil {
			return 0, execErr
		}

		return result.RowsAffected()

	default:
		panic(fmt.Sprintf("unsupported wrapper type: %T", e))
	}
}

func OptimizeDBWhileBenchmarking(wrapper Wrapper) error {
	query := `VACUUM ANALYZE EVENTS`

	switch e := wrapper.(type) {
	case *PGXPoolWrapper:
		_, execErr := e.pool.Exec(context.Background(), query)
		if execErr != nil {
			return execErr
		}

		return nil

	case *SQLDBWrapper:
		_, execErr := e.db.Exec(query)
		if execErr != nil {
			return execErr
		}

		return nil

	case *SQLXWrapper:
		_, execErr := e.db.Exec(query)
		if execErr != nil {
			return execErr
		}

		return nil

	default:
		panic(fmt.Sprintf("unsupported wrapper type: %T", e))
	}
}
