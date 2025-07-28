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
	return &sqlRows{rows: rows}, nil
}

func (s *SQLAdapter) Exec(ctx context.Context, query string) (DBResult, error) {
	result, err := s.db.ExecContext(ctx, query)
	if err != nil {
		return nil, err
	}
	return &sqlResult{result: result}, nil
}

type sqlRows struct {
	rows *sql.Rows
}

func (s *sqlRows) Next() bool {
	return s.rows.Next()
}

func (s *sqlRows) Scan(dest ...interface{}) error {
	return s.rows.Scan(dest...)
}

func (s *sqlRows) Close() error {
	return s.rows.Close()
}

type sqlResult struct {
	result sql.Result
}

func (s *sqlResult) RowsAffected() (int64, error) {
	return s.result.RowsAffected()
}
