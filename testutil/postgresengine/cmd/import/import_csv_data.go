package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell/config"
)

func main() {
	if err := ImportCSVData(); err != nil {
		log.Fatalf("Error importing CSV data: %v", err)
	}
}

func ImportCSVData() error {
	startTime := time.Now()

	fmt.Println("🚀 Starting CSV data import")
	fmt.Println("📄 Source: /fixtures/events.csv")
	fmt.Println("🎯 Target: PostgreSQL benchmark database (port 5433)")
	fmt.Println()

	ctx := context.Background()

	// Connect using the benchmark config (port 5433)
	fmt.Printf("🔗\tConnecting to database...")
	connPool, err := pgxpool.NewWithConfig(ctx, config.PostgresPGXPoolBenchmarkConfig())
	if err != nil {
		return fmt.Errorf("failed to create connection pool: %w", err)
	}
	defer connPool.Close()
	fmt.Println(" ✅")

	fmt.Printf("📦\tPreparing import transaction...")
	fmt.Println(" ✅")

	// Start transaction
	fmt.Printf("🔄\tStarting transaction...")
	tx, err := connPool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx) // Will be ignored if already committed
	}()
	fmt.Println(" ✅")

	// Lock table
	fmt.Printf("🔒\tLocking events table...")
	_, err = tx.Exec(ctx, "LOCK TABLE events IN ACCESS EXCLUSIVE MODE")
	if err != nil {
		return fmt.Errorf("failed to lock table: %w", err)
	}
	fmt.Println(" ✅")

	// Drop indexes
	fmt.Printf("🗑️\tDropping indexes...")
	_, err = tx.Exec(ctx, `
		DROP INDEX IF EXISTS idx_events_occurred_at;
		DROP INDEX IF EXISTS idx_events_event_type;
		DROP INDEX IF EXISTS idx_events_payload_gin;
		DROP INDEX IF EXISTS idx_events_metadata_gin;
	`)
	if err != nil {
		return fmt.Errorf("failed to drop indexes: %w", err)
	}
	fmt.Println(" ✅")

	// Set performance parameters
	fmt.Printf("⚙️\tOptimizing performance settings...")
	_, err = tx.Exec(ctx, `
		SET LOCAL work_mem = '256MB';
		SET LOCAL maintenance_work_mem = '512MB';
		SET LOCAL synchronous_commit = off;
	`)
	if err != nil {
		return fmt.Errorf("failed to set performance settings: %w", err)
	}
	fmt.Println(" ✅")

	// Truncate table
	fmt.Printf("🧹\tClearing existing data...")
	_, err = tx.Exec(ctx, "TRUNCATE TABLE events RESTART IDENTITY")
	if err != nil {
		return fmt.Errorf("failed to truncate table: %w", err)
	}
	fmt.Println(" ✅")

	// Import CSV data
	fmt.Printf("📥\tImporting CSV data...")
	csvImportStart := time.Now()
	_, err = tx.Exec(ctx, `
		COPY events (occurred_at, event_type, payload, metadata)
		FROM '/fixtures/events.csv'
		WITH (FORMAT csv, FREEZE true)
	`)
	if err != nil {
		return fmt.Errorf("failed to import CSV: %w", err)
	}
	csvImportDuration := time.Since(csvImportStart)
	fmt.Printf(" ✅ %v\n", csvImportDuration.Round(time.Millisecond))

	// Recreate indexes
	fmt.Printf("🔧\tRecreating indexes...")
	indexStart := time.Now()
	_, err = tx.Exec(ctx, `
		CREATE INDEX idx_events_event_type ON events(event_type);
		CREATE INDEX idx_events_occurred_at ON events(occurred_at);
		CREATE INDEX idx_events_payload_gin ON events USING gin(payload jsonb_path_ops);
		CREATE INDEX idx_events_metadata_gin ON events USING gin(metadata jsonb_path_ops);
	`)
	if err != nil {
		return fmt.Errorf("failed to recreate indexes: %w", err)
	}
	indexDuration := time.Since(indexStart)
	fmt.Printf(" ✅ %v\n", indexDuration.Round(time.Millisecond))

	// Update statistics
	fmt.Printf("📊\tUpdating table statistics...")
	analyzeStart := time.Now()
	_, err = tx.Exec(ctx, "ANALYZE events")
	if err != nil {
		return fmt.Errorf("failed to analyze table: %w", err)
	}
	analyzeDuration := time.Since(analyzeStart)
	fmt.Printf(" ✅ %v\n", analyzeDuration.Round(time.Millisecond))

	// Commit transaction
	fmt.Printf("💾\tCommitting transaction...")
	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	fmt.Println(" ✅")

	// Verify the import
	fmt.Printf("🔍 Verifying import...")
	var count int
	err = connPool.QueryRow(ctx, "SELECT count(*) FROM events").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to verify import: %w", err)
	}
	fmt.Println(" ✅")

	elapsed := time.Since(startTime)

	fmt.Println()
	fmt.Printf("Import completed! 🎉\n")
	fmt.Printf("Total events imported: %s 📊\n", formatNumber(count))
	fmt.Printf("Total time: %v ⏱️\n", elapsed.Round(time.Millisecond))

	return nil
}

func formatNumber(n int) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000.0)
	} else if n >= 100000 {
		return fmt.Sprintf("%.0fK", float64(n)/1000)
	} else if n >= 10000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return strconv.Itoa(n)
}
