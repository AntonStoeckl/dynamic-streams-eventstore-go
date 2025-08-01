package adapters

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
)

// SQLXAdapter implements DBAdapter for sqlx.DB.
type SQLXAdapter struct {
	db *sqlx.DB
}

// NewSQLXAdapter creates a new SQLX adapter.
func NewSQLXAdapter(db *sqlx.DB) *SQLXAdapter {
	return &SQLXAdapter{db: db}
}

// Query executes a query using the sqlx.DB and returns wrapped rows.
func (s *SQLXAdapter) Query(ctx context.Context, query string) (DBRows, error) {
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		defer func(rows *sql.Rows) {
			_ = rows.Close()
		}(rows)

		return nil, eventstore.ErrRowsIterationFailed
	}

	return &stdRows{rows: rows}, nil
}

// Exec executes a query using the sqlx.DB and returns a wrapped result.
func (s *SQLXAdapter) Exec(ctx context.Context, query string) (DBResult, error) {
	result, err := s.db.ExecContext(ctx, query)
	if err != nil {
		return nil, err
	}

	return &stdResult{result: result}, nil
}
