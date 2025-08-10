package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/oteladapters"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell/config"
)

//nolint:funlen
func main() {
	cfg := parseFlags()

	ctx, cancel := context.WithCancel(context.Background())

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Get database adapter type from environment variable (default: pgx)
	adapterType := strings.ToLower(os.Getenv("DB_ADAPTER"))
	if adapterType == "" {
		adapterType = "pgx"
	}

	log.Printf("🔧 USING DATABASE ADAPTER: %s", strings.ToUpper(adapterType))

	// Initialize observability (if enabled)
	var eventStoreOptions []postgresengine.Option
	if cfg.ObservabilityEnabled {
		obsConfig := cfg.NewObservabilityConfig()
		if obsConfig.Logger != nil {
			eventStoreOptions = append(eventStoreOptions, postgresengine.WithLogger(obsConfig.Logger))
		}
		if obsConfig.ContextualLogger != nil {
			eventStoreOptions = append(eventStoreOptions, postgresengine.WithContextualLogger(obsConfig.ContextualLogger))
		}
		if obsConfig.MetricsCollector != nil {
			eventStoreOptions = append(eventStoreOptions, postgresengine.WithMetrics(obsConfig.MetricsCollector))
		}
		if obsConfig.TracingCollector != nil {
			eventStoreOptions = append(eventStoreOptions, postgresengine.WithTracing(obsConfig.TracingCollector))
		}
		log.Printf("Observability enabled: metrics=%v, tracing=%v, logging=%v",
			obsConfig.MetricsCollector != nil,
			obsConfig.TracingCollector != nil,
			obsConfig.Logger != nil || obsConfig.ContextualLogger != nil)
	}

	// Initialize EventStore based on the adapter type
	var eventStore *postgresengine.EventStore
	var err error

	switch adapterType {
	case "pgx":
		eventStore, err = initializePGXEventStore(ctx, eventStoreOptions...)
	case "sql", "sql.db":
		eventStore, err = initializeSQLDBEventStore(eventStoreOptions...)
	case "sqlx":
		eventStore, err = initializeSQLXEventStore(eventStoreOptions...)
	default:
		cancel()
		log.Fatalf("Unknown database adapter: %s (supported: pgx, sql, sqlx)", adapterType)
	}

	if err != nil {
		cancel()
		log.Fatalf("Failed to create EventStore: %v", err)
	}

	// Get observability config for command handlers
	obsConfig := cfg.NewObservabilityConfig()

	// Initialize library simulation
	simulation, err := NewLibrarySimulation(eventStore, cfg, obsConfig)
	if err != nil {
		cancel()
		log.Fatalf("Failed to create LibrarySimulation: %v", err)
	}

	// Start simulation in a goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := simulation.Start(ctx); err != nil {
			errChan <- fmt.Errorf("library simulation failed: %w", err)
		}
	}()

	log.Printf("Library Simulation started")
	log.Printf("Configuration: rate=%d req/s, books=%d-%d, readers=%d-%d, setup-sleep=%ds",
		cfg.Rate, cfg.MinBooks, cfg.MaxBooks, cfg.MinReaders, cfg.MaxReaders, cfg.SetupSleepSeconds)
	log.Printf("Error rates: idempotent=%.1f%%, manager-conflict=%.1f%%, reader-removed=%.1f%%",
		cfg.ErrorProbabilities.IdempotentRepeat, cfg.ErrorProbabilities.LibraryManagerBookConflict, cfg.ErrorProbabilities.ReaderBorrowRemovedBook)
	log.Printf("Press Ctrl+C to stop...")

	// Wait for a shutdown signal or error
	select {
	case sig := <-sigChan:
		log.Printf("Received signal %v, initiating graceful shutdown...", sig)
		cancel()
	case err := <-errChan:
		log.Printf("Error occurred: %v", err)
		cancel()
	}

	// Give some time for a graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := simulation.Stop(shutdownCtx); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	cancel()
	log.Printf("Library Simulation stopped")
}

func (c Config) NewObservabilityConfig() ObservabilityConfig {
	if !c.ObservabilityEnabled {
		return ObservabilityConfig{}
	}

	// Create real OpenTelemetry providers for the simulation
	_, err := config.NewTestObservabilityConfig()
	if err != nil {
		log.Printf("Failed to create observability providers: %v", err)
		return ObservabilityConfig{}
	}
	// Note: Providers are set globally in OpenTelemetry, no need to store reference

	// Create real OpenTelemetry adapters (same as the test)
	tracer := otel.Tracer("eventstore-library-simulation")
	meter := otel.Meter("eventstore-library-simulation")

	metricsCollector := oteladapters.NewMetricsCollector(meter)
	tracingCollector := oteladapters.NewTracingCollector(tracer)
	contextualLogger := oteladapters.NewSlogBridgeLogger("eventstore-library-simulation")

	return ObservabilityConfig{
		Logger:           nil, // Using contextual logger instead
		ContextualLogger: contextualLogger,
		MetricsCollector: metricsCollector,
		TracingCollector: tracingCollector,
	}
}

// initializePGXEventStore creates EventStore using pgx.Pool adapters.
func initializePGXEventStore(ctx context.Context, options ...postgresengine.Option) (*postgresengine.EventStore, error) {
	log.Printf("🔧 Initializing PGX adapter with connection pools")
	// Initialize primary connection
	pgxPoolPrimary, primaryErr := pgxpool.NewWithConfig(ctx, config.PostgresPGXPoolPrimaryConfig())
	if primaryErr != nil {
		return nil, fmt.Errorf("failed to create pgx pool for primary: %w", primaryErr)
	}

	// Test primary connection
	if pingPrimaryErr := pgxPoolPrimary.Ping(ctx); pingPrimaryErr != nil {
		pgxPoolPrimary.Close()
		return nil, fmt.Errorf("failed to connect to primary database: %w", pingPrimaryErr)
	}

	// Initialize replica connection
	pgxPoolReplica, replicaErr := pgxpool.NewWithConfig(ctx, config.PostgresPGXPoolReplicaConfig())
	if replicaErr != nil {
		pgxPoolPrimary.Close()
		return nil, fmt.Errorf("failed to create pgx pool for replica: %w", replicaErr)
	}

	// Test replica connection
	if pingReplicaErr := pgxPoolReplica.Ping(ctx); pingReplicaErr != nil {
		pgxPoolPrimary.Close()
		pgxPoolReplica.Close()
		return nil, fmt.Errorf("failed to connect to replica database: %w", pingReplicaErr)
	}

	return postgresengine.NewEventStoreFromPGXPoolAndReplica(pgxPoolPrimary, pgxPoolReplica, options...)
}

// initializeSQLDBEventStore creates EventStore using sql.DB adapters.
func initializeSQLDBEventStore(options ...postgresengine.Option) (*postgresengine.EventStore, error) {
	log.Printf("🔧 Initializing SQL.DB adapter with proper lib/pq driver")

	// Use the proper config functions (these use lib/pq driver, not pgx)
	primaryDB := config.PostgresSQLDBPrimaryConfig()
	replicaDB := config.PostgresSQLDBReplicaConfig()

	return postgresengine.NewEventStoreFromSQLDBAndReplica(primaryDB, replicaDB, options...)
}

// initializeSQLXEventStore creates EventStore using sqlx adapters.
func initializeSQLXEventStore(options ...postgresengine.Option) (*postgresengine.EventStore, error) {
	log.Printf("🔧 Initializing SQLX adapter with proper lib/pq driver")

	// Use the proper config functions (these use lib/pq driver, not pgx)
	primaryDB := config.PostgresSQLXPrimaryConfig()
	replicaDB := config.PostgresSQLXReplicaConfig()

	return postgresengine.NewEventStoreFromSQLXAndReplica(primaryDB, replicaDB, options...)
}
