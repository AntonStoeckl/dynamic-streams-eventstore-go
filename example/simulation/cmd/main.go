package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/oteladapters"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell/config"
)

func main() {
	cfg := parseFlags()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Initialize database connection using primary node config (port 5433)
	pgxPoolPrimary, primaryErr := pgxpool.NewWithConfig(ctx, config.PostgresPGXPoolPrimaryConfig())
	if primaryErr != nil {
		cancel()
		log.Fatalf("Failed to create pgx pool for primary: %v", primaryErr)
	}
	defer pgxPoolPrimary.Close()

	// Test database connection
	if pingPrimaryErr := pgxPoolPrimary.Ping(ctx); pingPrimaryErr != nil {
		cancel()
		log.Fatalf("Failed to connect to primary database: %v", pingPrimaryErr)
	}

	// Initialize database connection using replica node config (port 5434)
	pgxPoolReplica, replicaErr := pgxpool.NewWithConfig(ctx, config.PostgresPGXPoolReplicaConfig())
	if replicaErr != nil {
		cancel()
		log.Fatalf("Failed to create pgx pool for replica: %v", replicaErr)
	}
	defer pgxPoolReplica.Close()

	// Test database connection
	if pingReplicaErr := pgxPoolReplica.Ping(ctx); pingReplicaErr != nil {
		cancel()
		log.Fatalf("Failed to connect to replica database: %v", pingReplicaErr)
	}

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

	// Initialize EventStore
	eventStore, err := postgresengine.NewEventStoreFromPGXPoolAndReplica(pgxPoolPrimary, pgxPoolReplica, eventStoreOptions...)
	if err != nil {
		log.Fatalf("Failed to create EventStore: %v", err)
	}

	// Get observability config for command handlers
	obsConfig := cfg.NewObservabilityConfig()

	// Initialize library simulation
	simulation, err := NewLibrarySimulation(eventStore, cfg, obsConfig)
	if err != nil {
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
