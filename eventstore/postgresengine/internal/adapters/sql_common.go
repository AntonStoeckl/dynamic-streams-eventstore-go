package adapters

import "database/sql"

// stdRows wraps standard library sql.Rows to implement DBRows interface
type stdRows struct {
	rows *sql.Rows
}

func (s *stdRows) Next() bool {
	return s.rows.Next()
}

func (s *stdRows) Scan(dest ...any) error {
	return s.rows.Scan(dest...)
}

func (s *stdRows) Close() error {
	return s.rows.Close()
}

// stdResult wraps standard library sql.Result to implement DBResult interface
type stdResult struct {
	result sql.Result
}

func (s *stdResult) RowsAffected() (int64, error) {
	return s.result.RowsAffected()
}
