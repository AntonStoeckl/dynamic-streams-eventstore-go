package adapters

import (
	"context"

	"github.com/jmoiron/sqlx"
)

// SQLXAdapter implements DBAdapter for sqlx.DB
type SQLXAdapter struct {
	db *sqlx.DB
}

// NewSQLXAdapter creates a new SQLX adapter
func NewSQLXAdapter(db *sqlx.DB) *SQLXAdapter {
	return &SQLXAdapter{db: db}
}

func (s *SQLXAdapter) Query(ctx context.Context, query string) (DBRows, error) {
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	return &sqlRows{rows: rows}, nil
}

func (s *SQLXAdapter) Exec(ctx context.Context, query string) (DBResult, error) {
	result, err := s.db.ExecContext(ctx, query)
	if err != nil {
		return nil, err
	}
	return &sqlResult{result: result}, nil
}