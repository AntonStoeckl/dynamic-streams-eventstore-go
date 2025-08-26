package adapters

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
)

// PGXAdapter implements DBAdapter for pgxpool.Pool.
type PGXAdapter struct {
	pool        *pgxpool.Pool
	replicaPool *pgxpool.Pool // optional replica for read operations
}

// NewPGXAdapter creates a new PGX adapter with a primary pool.
func NewPGXAdapter(pool *pgxpool.Pool) *PGXAdapter {
	return &PGXAdapter{pool: pool}
}

// NewPGXAdapterWithReplica creates a new PGX adapter with a primary pool and a replica pool.
func NewPGXAdapterWithReplica(pool *pgxpool.Pool, replica *pgxpool.Pool) *PGXAdapter {
	return &PGXAdapter{pool: pool, replicaPool: replica}
}

// Query executes a query using consistency-aware routing.
// By default, uses primary pool for strong consistency (safe for event sourcing).
// Uses replica pool only when context explicitly requests eventual consistency.
func (p *PGXAdapter) Query(ctx context.Context, query string) (DBRows, error) {
	pool := p.pool // default to primary for strong consistency

	// Only use replica when explicitly requesting eventual consistency
	if p.replicaPool != nil && eventstore.GetConsistencyLevel(ctx) == eventstore.EventualConsistency {
		pool = p.replicaPool
	}

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	return &pgxRows{rows: rows}, nil
}

// QueryRow executes a query that returns a single row using consistency-aware routing.
// By default, uses primary pool for strong consistency (safe for event sourcing).
// Uses replica pool only when context explicitly requests eventual consistency.
func (p *PGXAdapter) QueryRow(ctx context.Context, query string) DBRow {
	pool := p.pool // default to primary for strong consistency

	// Only use replica when explicitly requesting eventual consistency
	if p.replicaPool != nil && eventstore.GetConsistencyLevel(ctx) == eventstore.EventualConsistency {
		pool = p.replicaPool
	}

	row := pool.QueryRow(ctx, query)
	return &pgxRow{row: row}
}

// Exec executes a query using the pgx pool and returns a wrapped result.
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

// Err returns any error encountered during iteration.
func (p *pgxRows) Err() error {
	return p.rows.Err()
}

// pgxRow wraps pgx.Row to implement the DBRow interface.
type pgxRow struct {
	row pgx.Row
}

// Scan copies row values into provided destinations.
func (p *pgxRow) Scan(dest ...any) error {
	return p.row.Scan(dest...)
}

// pgxResult wraps pgconn.CommandTag to implement the DBResult interface.
type pgxResult struct {
	tag pgconn.CommandTag
}

// RowsAffected returns the number of rows affected by the command.
func (p *pgxResult) RowsAffected() (int64, error) {
	return p.tag.RowsAffected(), nil
}
