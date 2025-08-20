package adapters

import (
	"context"
	"database/sql"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
)

// SQLAdapter implements DBAdapter for sql.DB.
type SQLAdapter struct {
	db        *sql.DB
	replicaDB *sql.DB // optional replica for read operations
}

// NewSQLAdapter creates a new SQL adapter with a primary db connection.
func NewSQLAdapter(db *sql.DB) *SQLAdapter {
	return &SQLAdapter{db: db}
}

// NewSQLAdapterWithReplica creates a new SQL adapter with a primary
// db connection and a replica db connection.
func NewSQLAdapterWithReplica(db *sql.DB, replicaDB *sql.DB) *SQLAdapter {
	return &SQLAdapter{db: db, replicaDB: replicaDB}
}

// Query executes a query using the replica database if available, otherwise the primary database.
func (s *SQLAdapter) Query(ctx context.Context, query string) (DBRows, error) {
	db := s.db // default to primary

	if s.replicaDB != nil {
		db = s.replicaDB // use replica for reads
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

// QueryRow executes a query that returns a single row using the replica database if available.
func (s *SQLAdapter) QueryRow(ctx context.Context, query string) DBRow {
	db := s.db // default to primary

	if s.replicaDB != nil {
		db = s.replicaDB // use replica for reads
	}

	row := db.QueryRowContext(ctx, query)
	return &stdRow{row: row}
}

// Exec executes a query using the sql.DB and returns a wrapped result.
func (s *SQLAdapter) Exec(ctx context.Context, query string) (DBResult, error) {
	result, err := s.db.ExecContext(ctx, query)
	if err != nil {
		return nil, err
	}

	return &stdResult{result: result}, nil
}
