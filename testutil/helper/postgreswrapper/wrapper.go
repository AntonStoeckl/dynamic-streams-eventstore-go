package postgreswrapper

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"

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

// CreateWrapper creates the appropriate wrapper based on the environment variable
func CreateWrapper(t testing.TB) Wrapper {
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

	default: // typePGXPool
		panic(fmt.Sprintf("unsupported engine type from env: %s", engineTypeFromEnv))
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
		t.Fatalf("unsupported wrapper type: %T", e)
	}
}
