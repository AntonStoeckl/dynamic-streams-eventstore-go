package adapters

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PGXAdapter implements DBAdapter for pgxpool.Pool
type PGXAdapter struct {
	pool *pgxpool.Pool
}

// NewPGXAdapter creates a new PGX adapter
func NewPGXAdapter(pool *pgxpool.Pool) *PGXAdapter {
	return &PGXAdapter{pool: pool}
}

// Query executes a query using the pgx pool and returns wrapped rows.
func (p *PGXAdapter) Query(ctx context.Context, query string) (DBRows, error) {
	rows, err := p.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	return &pgxRows{rows: rows}, nil
}

// Exec executes a query using the pgx pool and returns wrapped result.
func (p *PGXAdapter) Exec(ctx context.Context, query string) (DBResult, error) {
	tag, err := p.pool.Exec(ctx, query)
	if err != nil {
		return nil, err
	}
	return &pgxResult{tag: tag}, nil
}

// pgxRows wraps pgx.Rows to implement the DBRows interface.
type pgxRows struct {
	rows pgx.Rows
}

// Next advances to the next row.
func (p *pgxRows) Next() bool {
	return p.rows.Next()
}

// Scan copies row values into provided destinations.
func (p *pgxRows) Scan(dest ...any) error {
	return p.rows.Scan(dest...)
}

// Close closes the rows iterator.
func (p *pgxRows) Close() error {
	p.rows.Close()
	return nil
}

// pgxResult wraps pgconn.CommandTag to implement the DBResult interface.
type pgxResult struct {
	tag pgconn.CommandTag
}

// RowsAffected returns the number of rows affected by the command.
func (p *pgxResult) RowsAffected() (int64, error) {
	return p.tag.RowsAffected(), nil
}
