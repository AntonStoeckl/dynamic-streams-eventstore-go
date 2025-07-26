package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/test/userland/config"
)

func main() {
	if err := ImportCSVData(); err != nil {
		log.Fatalf("Error importing CSV data: %v", err)
	}
}

func ImportCSVData() error {
	ctx := context.Background()

	// Connect using the benchmark config (port 5433)
	connPool, err := pgxpool.NewWithConfig(ctx, config.PostgresBenchmarkConfig())
	if err != nil {
		return fmt.Errorf("failed to create connection pool: %w", err)
	}
	defer connPool.Close()

	fmt.Println("Starting CSV import transaction...")

	// Execute the entire SQL script as one transaction
	sqlScript := `
-- Step 1: Start a transaction
BEGIN;

-- Step 2: Lock the table to prevent other operations
LOCK TABLE events IN ACCESS EXCLUSIVE MODE;

-- Step 3: Drop indexes
DROP INDEX IF EXISTS idx_events_occurred_at;
DROP INDEX IF EXISTS idx_events_event_type;
DROP INDEX IF EXISTS idx_events_payload_gin;
DROP INDEX IF EXISTS idx_events_metadata_gin;

-- Step 4: Increase work memory for better performance
SET LOCAL work_mem = '256MB';

-- Step 5: Set maintenance work memory for index creation
SET LOCAL maintenance_work_mem = '512MB';

-- Step 6: Disable synchronous commit for this transaction (faster writes)
SET LOCAL synchronous_commit = off;

-- Step 7: Truncate table to start fresh (optional)
TRUNCATE TABLE events RESTART IDENTITY;

-- Step 8: Import the CSV
COPY events (occurred_at, event_type, payload, metadata)
    FROM '/fixtures/events.csv'
    WITH (FORMAT csv, FREEZE true);

-- Step 9: Recreate indexes
CREATE INDEX idx_events_event_type ON events(event_type);
CREATE INDEX idx_events_occurred_at ON events(occurred_at);
CREATE INDEX idx_events_payload_gin ON events USING gin(payload jsonb_path_ops);
CREATE INDEX idx_events_metadata_gin ON events USING gin(metadata jsonb_path_ops);

-- Step 10: Update table statistics
ANALYZE events;

-- Step 11: Commit the transaction
COMMIT;
	`

	fmt.Println("Executing import script...")
	_, err = connPool.Exec(ctx, sqlScript)
	if err != nil {
		return fmt.Errorf("failed to execute import script: %w", err)
	}

	// Verify the import
	fmt.Println("Verifying import...")
	var count int
	err = connPool.QueryRow(ctx, "SELECT count(*) FROM events").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to verify import: %w", err)
	}

	fmt.Printf("Successfully imported %d events from CSV\n", count)
	return nil
}
