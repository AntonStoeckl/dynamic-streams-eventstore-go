package adapters

import "context"

// DBAdapter defines the interface for database operations needed by the event store
type DBAdapter interface {
	Query(ctx context.Context, query string) (DBRows, error)
	Exec(ctx context.Context, query string) (DBResult, error)
}

// DBRows defines the interface for query result rows
type DBRows interface {
	Next() bool
	Scan(dest ...any) error
	Close() error
}

// DBResult defines the interface for execution results
type DBResult interface {
	RowsAffected() (int64, error)
}
