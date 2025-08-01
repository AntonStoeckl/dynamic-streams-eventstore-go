package adapters

import (
	"context"
	"database/sql"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
)

// SQLAdapter implements DBAdapter for sql.DB.
type SQLAdapter struct {
	db *sql.DB
}

// NewSQLAdapter creates a new SQL adapter.
func NewSQLAdapter(db *sql.DB) *SQLAdapter {
	return &SQLAdapter{db: db}
}

// Query executes a query using the sql.DB and returns wrapped rows.
func (s *SQLAdapter) Query(ctx context.Context, query string) (DBRows, error) {
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		_ = rows.Close()
		return nil, eventstore.ErrRowsIterationFailed
	}

	return &stdRows{rows: rows}, nil
}

// Exec executes a query using the sql.DB and returns a wrapped result.
func (s *SQLAdapter) Exec(ctx context.Context, query string) (DBResult, error) {
	result, err := s.db.ExecContext(ctx, query)
	if err != nil {
		return nil, err
	}

	return &stdResult{result: result}, nil
}
