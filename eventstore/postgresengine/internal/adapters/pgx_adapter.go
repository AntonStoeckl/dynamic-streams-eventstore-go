package adapters

import (
	"context"

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

func (p *PGXAdapter) Query(ctx context.Context, query string) (DBRows, error) {
	rows, err := p.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	return &pgxRows{rows: rows}, nil
}

func (p *PGXAdapter) Exec(ctx context.Context, query string) (DBResult, error) {
	tag, err := p.pool.Exec(ctx, query)
	if err != nil {
		return nil, err
	}
	return &pgxResult{tag: tag}, nil
}

type pgxRows struct {
	rows interface {
		Next() bool
		Scan(dest ...interface{}) error
		Close()
	}
}

func (p *pgxRows) Next() bool {
	return p.rows.Next()
}

func (p *pgxRows) Scan(dest ...interface{}) error {
	return p.rows.Scan(dest...)
}

func (p *pgxRows) Close() error {
	p.rows.Close()
	return nil
}

type pgxResult struct {
	tag interface {
		RowsAffected() int64
	}
}

func (p *pgxResult) RowsAffected() (int64, error) {
	return p.tag.RowsAffected(), nil
}
