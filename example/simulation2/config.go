package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/oteladapters"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell/config"
)

// Config holds command-line configuration for the actor-based simulation.
type Config struct {
	ObservabilityEnabled bool
	Workers              int
	MaxActiveReaders     int
}

// ObservabilityConfig holds the observability adapters for command handlers.
type ObservabilityConfig struct {
	Logger           eventstore.Logger
	ContextualLogger eventstore.ContextualLogger
	MetricsCollector eventstore.MetricsCollector
	TracingCollector eventstore.TracingCollector
}

func parseFlags() Config {
	var (
		observability    = flag.Bool("observability-enabled", false, "Enable OpenTelemetry observability")
		workers          = flag.Int("workers", DefaultConcurrentWorkers, "Number of concurrent workers")
		maxActiveReaders = flag.Int("max-readers", DefaultMaxActiveReaders, "Maximum number of active readers")
	)

	flag.Parse()

	return Config{
		ObservabilityEnabled: *observability,
		Workers:              *workers,
		MaxActiveReaders:     *maxActiveReaders,
	}
}

func (c Config) newObservabilityConfig() ObservabilityConfig {
	if !c.ObservabilityEnabled {
		return ObservabilityConfig{}
	}

	// Create real OpenTelemetry providers for the simulation.
	_, err := config.NewObservabilityConfig()
	if err != nil {
		log.Printf("Failed to create observability providers: %v", err)
		return ObservabilityConfig{}
	}
	// Note: Providers are set globally in OpenTelemetry, no need to store reference.

	// Create real OpenTelemetry adapters (same as the test).
	tracer := otel.Tracer("eventstore-library-simulation-v2")
	meter := otel.Meter("eventstore-library-simulation-v2")

	metricsCollector := oteladapters.NewMetricsCollector(meter)
	tracingCollector := oteladapters.NewTracingCollector(tracer)
	contextualLogger := oteladapters.NewSlogBridgeLogger("eventstore-library-simulation-v2")

	return ObservabilityConfig{
		Logger:           nil, // Using contextual logger instead.
		ContextualLogger: contextualLogger,
		MetricsCollector: metricsCollector,
		TracingCollector: tracingCollector,
	}
}

func initializePGXEventStore(ctx context.Context, cfg Config) (*postgresengine.EventStore, error) {
	log.Printf("%s %s", Info("🔧"), Info("Initializing PGX adapter with connection pools"))

	// Initialize primary connection.
	pgxPoolPrimary, primaryErr := pgxpool.NewWithConfig(ctx, config.PostgresPGXPoolPrimaryConfig())
	if primaryErr != nil {
		return nil, fmt.Errorf("failed to create pgx pool for primary: %w", primaryErr)
	}

	// Test primary connection.
	if pingPrimaryErr := pgxPoolPrimary.Ping(ctx); pingPrimaryErr != nil {
		pgxPoolPrimary.Close()
		return nil, fmt.Errorf("failed to connect to primary database: %w", pingPrimaryErr)
	}

	// Initialize replica connection.
	pgxPoolReplica, replicaErr := pgxpool.NewWithConfig(ctx, config.PostgresPGXPoolReplicaConfig())
	if replicaErr != nil {
		pgxPoolPrimary.Close()
		return nil, fmt.Errorf("failed to create pgx pool for replica: %w", replicaErr)
	}

	// Test replica connection.
	if pingReplicaErr := pgxPoolReplica.Ping(ctx); pingReplicaErr != nil {
		pgxPoolPrimary.Close()
		pgxPoolReplica.Close()
		return nil, fmt.Errorf("failed to connect to replica database: %w", pingReplicaErr)
	}

	// Set up EventStore observability options if enabled.
	var eventStoreOptions []postgresengine.Option
	if cfg.ObservabilityEnabled {
		obsConfig := cfg.newObservabilityConfig() //nolint:contextcheck // Initialization code, context created internally
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
		log.Printf("🔍 EventStore observability enabled: metrics=%v, tracing=%v, logging=%v",
			obsConfig.MetricsCollector != nil,
			obsConfig.TracingCollector != nil,
			obsConfig.Logger != nil || obsConfig.ContextualLogger != nil)
	}

	return postgresengine.NewEventStoreFromPGXPoolAndReplica(pgxPoolPrimary, pgxPoolReplica, eventStoreOptions...)
}
