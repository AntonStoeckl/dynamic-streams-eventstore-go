package adapters

import "database/sql"

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
