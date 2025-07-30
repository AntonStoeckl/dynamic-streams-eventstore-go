package adapters

import (
	"context"
	"database/sql"
)

// SQLAdapter implements DBAdapter for sql.DB
type SQLAdapter struct {
	db *sql.DB
}

// NewSQLAdapter creates a new SQL adapter
func NewSQLAdapter(db *sql.DB) *SQLAdapter {
	return &SQLAdapter{db: db}
}

func (s *SQLAdapter) Query(ctx context.Context, query string) (DBRows, error) {
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	return &stdRows{rows: rows}, nil
}

func (s *SQLAdapter) Exec(ctx context.Context, query string) (DBResult, error) {
	result, err := s.db.ExecContext(ctx, query)
	if err != nil {
		return nil, err
	}
	return &stdResult{result: result}, nil
}
