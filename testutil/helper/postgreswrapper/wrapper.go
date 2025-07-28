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
	"github.com/stretchr/testify/assert"

	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/config"
)

// Engine type constants
const (
	typePGXPool = "pgxpool"
	typeSQLDB   = "sqldb"
)

// Wrapper interface to abstract over different engine types
type Wrapper interface {
	GetEventStore() EventStore
	Close()
}

// PGXPoolWrapper wraps pgxpool-based testing
type PGXPoolWrapper struct {
	pool *pgxpool.Pool
	es   EventStore
}

func (e *PGXPoolWrapper) GetEventStore() EventStore {
	return e.es
}

func (e *PGXPoolWrapper) Close() {
	e.pool.Close()
}

// SQLDBWrapper wraps sql.DB-based testing
type SQLDBWrapper struct {
	db *sql.DB
	es EventStore
}

func (e *SQLDBWrapper) GetEventStore() EventStore {
	return e.es
}

func (e *SQLDBWrapper) Close() {
	_ = e.db.Close() // ignore error
}

// CreateWrapperWithTestConfig creates the appropriate wrapper based on the environment variable
func CreateWrapperWithTestConfig(t testing.TB) Wrapper {
	engineTypeFromEnv := strings.ToLower(os.Getenv("ADAPTER_TYPE"))

	switch engineTypeFromEnv {
	case typePGXPool, "":
		connPool, err := pgxpool.NewWithConfig(context.Background(), config.PostgresPGXPoolTestConfig())
		assert.NoError(t, err, "error connecting to DB pool in test setup")
		es := NewEventStoreFromPGXPool(connPool)

		return &PGXPoolWrapper{pool: connPool, es: es}

	case typeSQLDB:
		db := config.PostgresSQLDBTestConfig()
		es := NewEventStoreFromSQLDB(db)

		return &SQLDBWrapper{db: db, es: es}

	default: // neither one of the known types nor empty
		panic(fmt.Sprintf("unsupported wrapper type from env: %s", engineTypeFromEnv))
	}
}

// CreateWrapperWithBenchmarkConfig creates the appropriate wrapper based on the environment variable
func CreateWrapperWithBenchmarkConfig(t testing.TB) Wrapper {
	engineTypeFromEnv := strings.ToLower(os.Getenv("ADAPTER_TYPE"))

	switch engineTypeFromEnv {
	case typePGXPool, "":
		connPool, err := pgxpool.NewWithConfig(context.Background(), config.PostgresPGXPoolBenchmarkConfig())
		assert.NoError(t, err, "error connecting to DB pool in test setup")
		es := NewEventStoreFromPGXPool(connPool)

		return &PGXPoolWrapper{pool: connPool, es: es}

	case typeSQLDB:
		db := config.PostgresSQLDBBenchmarkConfig()
		es := NewEventStoreFromSQLDB(db)

		return &SQLDBWrapper{db: db, es: es}

	default: // neither one of the known types nor empty
		panic(fmt.Sprintf("unsupported wrapper type from env: %s", engineTypeFromEnv))
	}
}

// CleanUp cleans up the events table for the given wrapper
func CleanUp(t testing.TB, wrapper Wrapper) {
	switch e := wrapper.(type) {
	case *PGXPoolWrapper:
		_, err := e.pool.Exec(context.Background(), "TRUNCATE TABLE events RESTART IDENTITY")
		assert.NoError(t, err, "error cleaning up the events table")

	case *SQLDBWrapper:
		_, err := e.db.Exec("TRUNCATE TABLE events RESTART IDENTITY")
		assert.NoError(t, err, "error cleaning up the events table")

	default:
		panic(fmt.Sprintf("unsupported wrapper type: %T", e))
	}
}

// GetGreatestOccurredAtTimeFromDB gets the maximum occurred_at time from the events table for the given wrapper
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

	default:
		panic(fmt.Sprintf("unsupported wrapper type: %T", e))
	}

	assert.NoError(t, err, "error in arranging test data")
	return greatestOccurredAtTime
}

// GetLatestBookIDFromDB gets the latest BookID from the events table for the given wrapper
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

	default:
		panic(fmt.Sprintf("unsupported wrapper type: %T", e))
	}

	assert.NoError(t, err, "error in arranging test data")
	assert.NotEmpty(t, bookID, "error in arranging test data")
	return bookID
}

// GuardThatThereAreEnoughFixtureEventsInStore checks if there are enough fixture events in the store for the given wrapper
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

// CleanUpBookEvents executes SQL for benchmark cleanup for the given wrapper
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

	default:
		panic(fmt.Sprintf("unsupported wrapper type: %T", e))
	}
}

// OptimizeDBForWhileBenchmarking executes SQL for benchmark cleanup for the given wrapper
func OptimizeDBForWhileBenchmarking(wrapper Wrapper) error {
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

	default:
		panic(fmt.Sprintf("unsupported wrapper type: %T", e))
	}
}
