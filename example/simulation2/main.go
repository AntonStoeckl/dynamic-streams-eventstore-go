package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	CPUProfile           string
	MemProfile           string
	// Note: No rate parameter - actors determine their own pace.
}

// ObservabilityConfig holds the observability adapters for command handlers.
type ObservabilityConfig struct {
	Logger           eventstore.Logger
	ContextualLogger eventstore.ContextualLogger
	MetricsCollector eventstore.MetricsCollector
	TracingCollector eventstore.TracingCollector
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("Simulation failed: %v", err)
	}
}

func run() error {
	log.Printf("🎭 Starting Actor-Based Library Simulation v2")

	cfg := parseFlags()

	// Database adapter configuration (reuse from v1).
	adapterType := os.Getenv("DB_ADAPTER")
	if adapterType == "" {
		adapterType = "pgx"
	}
	log.Printf("🔧 Using database adapter: %s", adapterType)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling for graceful shutdown.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Initialize EventStore (start with PGX, expand later).
	eventStore, err := initializePGXEventStore(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to create EventStore: %w", err)
	}

	logSimulationConfiguration()

	// Initialize simulation components.
	handlers, scheduler, loadController, err := initializeSimulationComponents(ctx, eventStore, cfg)
	if err != nil {
		return err
	}

	log.Printf("⏳ Simulation will start in %d seconds...", SetupPhaseDelaySeconds)
	time.Sleep(time.Duration(SetupPhaseDelaySeconds) * time.Second)

	log.Printf("🚀 Actor-based simulation starting...")
	log.Printf("💡 Actors will work at system's natural pace (no fixed rate)")
	log.Printf("Press Ctrl+C to stop...")

	// Start all simulation components.
	simulationCtx, simulationCancel := context.WithCancel(ctx)
	defer simulationCancel()

	if err := scheduler.Start(); err != nil {
		return fmt.Errorf("failed to start scheduler: %w", err)
	}

	if err := loadController.Start(); err != nil {
		return fmt.Errorf("failed to start load controller: %w", err)
	}

	// Store references for shutdown.
	_ = handlers // Will be used when actors execute commands.

	// Wait for a shutdown signal.
	select {
	case sig := <-sigChan:
		log.Printf("📢 Received signal %v, initiating graceful shutdown...", sig)
		simulationCancel()
	case <-simulationCtx.Done():
		log.Printf("📢 Simulation context cancelled")
	}

	// Graceful shutdown.
	gracefulShutdown(loadController, scheduler)

	return nil
}

// parseFlags parses command line flags.
func parseFlags() Config {
	var (
		observability = flag.Bool("observability-enabled", false, "Enable OpenTelemetry observability")
		cpuProfile    = flag.String("cpuprofile", "", "write cpu profile to file")
		memProfile    = flag.String("memprofile", "", "write memory profile to file")
	)

	flag.Parse()

	return Config{
		ObservabilityEnabled: *observability,
		CPUProfile:           *cpuProfile,
		MemProfile:           *memProfile,
	}
}

// initializePGXEventStore creates EventStore using pgx.Pool adapters with observability.
func initializePGXEventStore(ctx context.Context, cfg Config) (*postgresengine.EventStore, error) {
	log.Printf("🔧 Initializing PGX adapter with connection pools")

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

	// Setup EventStore observability options if enabled.
	var eventStoreOptions []postgresengine.Option
	if cfg.ObservabilityEnabled {
		obsConfig := cfg.NewObservabilityConfig() //nolint:contextcheck // Initialization code, context created internally
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

// NewObservabilityConfig creates observability configuration for the simulation.
func (c Config) NewObservabilityConfig() ObservabilityConfig {
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

// logSimulationConfiguration prints the configuration parameters.
func logSimulationConfiguration() {
	log.Printf("📊 Simulation Configuration:")
	log.Printf("  - Active Readers: %d (initial), %d-%d (auto-tuned)",
		InitialActiveReaders, MinActiveReaders, MaxActiveReaders)
	log.Printf("  - Population: %d-%d readers, %d-%d books",
		MinReaders, MaxReaders, MinBooks, MaxBooks)
	log.Printf("  - Batch Size: %d actors per goroutine", ActorBatchSize)
	log.Printf("  - Auto-tuning: P50<%dms, P99<%dms target",
		TargetP50LatencyMs, TargetP99LatencyMs)
}

// initializeSimulationComponents creates and connects all simulation components.
func initializeSimulationComponents(ctx context.Context, eventStore *postgresengine.EventStore, cfg Config) (*HandlerBundle, *ActorScheduler, *LoadController, error) {
	log.Printf("🏗️  Initializing simulation components...")

	// Create handlers for all library operations.
	handlers, err := NewHandlerBundle(eventStore, cfg) //nolint:contextcheck // Initialization code, context created internally
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create handler bundle: %w", err)
	}

	// Create the simulation state for fast lookups.
	state := NewSimulationState()

	// Create the actor scheduler with batch processing.
	scheduler := NewActorScheduler(ctx, eventStore, state, handlers)

	// Create load controller for auto-tuning.
	loadController := NewLoadController(ctx, scheduler, state)

	// Connect load controller to handlers for metrics collection.
	handlers.SetLoadController(loadController)

	// Connect simulation state to handlers for actor decisions.
	handlers.SetSimulationState(state)

	return handlers, scheduler, loadController, nil
}

// gracefulShutdown stops simulation components in reverse order.
func gracefulShutdown(loadController *LoadController, scheduler *ActorScheduler) {
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	log.Printf("🔄 Shutting down simulation...")

	// Stop components in reverse order.
	if err := loadController.Stop(); err != nil {
		log.Printf("⚠️  Error stopping load controller: %v", err)
	}

	if err := scheduler.Stop(); err != nil {
		log.Printf("⚠️  Error stopping scheduler: %v", err)
	}

	select {
	case <-shutdownCtx.Done():
		log.Printf("⚠️  Shutdown timeout exceeded")
	default:
		log.Printf("✅ Actor-Based Library Simulation v2 stopped gracefully")
	}
}
