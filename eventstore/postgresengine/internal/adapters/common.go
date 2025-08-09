package adapters

import (
	"context"
	"database/sql"
)

// DBAdapter defines the interface for database operations needed by the event store.
type DBAdapter interface {
	Query(ctx context.Context, query string) (DBRows, error)
	Exec(ctx context.Context, query string) (DBResult, error)
}

// DBRows defines the interface for query result rows.
type DBRows interface {
	Next() bool
	Scan(dest ...any) error
	Close() error
}

// DBResult defines the interface for execution results.
type DBResult interface {
	RowsAffected() (int64, error)
}

// stdRows wraps standard library sql.Rows to implement DBRows interface.
type stdRows struct {
	rows *sql.Rows
}

// Next advances to the next row.
func (s *stdRows) Next() bool {
	return s.rows.Next()
}

// Scan copies row values into provided destinations.
func (s *stdRows) Scan(dest ...any) error {
	return s.rows.Scan(dest...)
}

// Close closes the rows iterator.
func (s *stdRows) Close() error {
	return s.rows.Close()
}

// stdResult wraps standard library sql.Result to implement DBResult interface.
type stdResult struct {
	result sql.Result
}

// RowsAffected returns the number of rows affected by the command.
func (s *stdResult) RowsAffected() (int64, error) {
	return s.result.RowsAffected()
}
