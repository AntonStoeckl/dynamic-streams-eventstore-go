package adapters

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
)

// SQLXAdapter implements DBAdapter for sqlx.DB.
type SQLXAdapter struct {
	db        *sqlx.DB
	replicaDB *sqlx.DB // optional replica for read operations
}

// NewSQLXAdapter creates a new SQLX adapter with a primary db connection.
func NewSQLXAdapter(db *sqlx.DB) *SQLXAdapter {
	return &SQLXAdapter{db: db}
}

// NewSQLXAdapterWithReplica creates a new SQLX adapter with a primary
// db connection and a replica db connection.
func NewSQLXAdapterWithReplica(db *sqlx.DB, replicaDB *sqlx.DB) *SQLXAdapter {
	return &SQLXAdapter{db: db, replicaDB: replicaDB}
}

// Query executes a query using consistency-aware routing.
// By default, uses primary pool for strong consistency (safe for event sourcing).
// Uses replica pool only when context explicitly requests eventual consistency.
func (s *SQLXAdapter) Query(ctx context.Context, query string) (DBRows, error) {
	db := s.db // default to primary for strong consistency

	// Only use replica when explicitly requesting eventual consistency
	if s.replicaDB != nil && eventstore.GetConsistencyLevel(ctx) == eventstore.EventualConsistency {
		db = s.replicaDB
	}

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		defer func(rows *sql.Rows) {
			_ = rows.Close()
		}(rows)

		return nil, eventstore.ErrRowIterationFailed
	}

	return &stdRows{rows: rows}, nil
}

// QueryRow executes a query that returns a single row using consistency-aware routing.
// By default, uses primary pool for strong consistency (safe for event sourcing).
// Uses replica pool only when context explicitly requests eventual consistency.
func (s *SQLXAdapter) QueryRow(ctx context.Context, query string) DBRow {
	db := s.db // default to primary for strong consistency

	// Only use replica when explicitly requesting eventual consistency
	if s.replicaDB != nil && eventstore.GetConsistencyLevel(ctx) == eventstore.EventualConsistency {
		db = s.replicaDB
	}

	row := db.QueryRowContext(ctx, query)
	return &stdRow{row: row}
}

// Exec executes a query using the sqlx.DB and returns a wrapped result.
func (s *SQLXAdapter) Exec(ctx context.Context, query string) (DBResult, error) {
	result, err := s.db.ExecContext(ctx, query)
	if err != nil {
		return nil, err
	}

	return &stdResult{result: result}, nil
}
