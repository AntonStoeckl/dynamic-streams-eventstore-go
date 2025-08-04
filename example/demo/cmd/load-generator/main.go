package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/oteladapters"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell/config"
)

const (
	defaultRate            = 30
	defaultInitialBooks    = 1000
	defaultScenarioWeights = "20,80" // circulation, lending
)

type Config struct {
	Rate                 int
	ObservabilityEnabled bool
	InitialBooks         int
	ScenarioWeights      []int
	StateSyncIntervalSec int
}

func main() {
	cfg := parseFlags()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Initialize database connection using benchmark config (port 5433)
	pgxPool, err := pgxpool.NewWithConfig(ctx, config.PostgresPGXPoolBenchmarkConfig())
	if err != nil {
		log.Fatalf("Failed to create pgx pool: %v", err)
	}
	defer pgxPool.Close()

	// Test database connection
	if err := pgxPool.Ping(ctx); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
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
	eventStore, err := postgresengine.NewEventStoreFromPGXPool(pgxPool, eventStoreOptions...)
	if err != nil {
		log.Fatalf("Failed to create EventStore: %v", err)
	}

	// Initialize load generator (EventStore observability is configured above)
	loadGen := NewLoadGenerator(eventStore, cfg)

	// Start load generation in a goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := loadGen.Start(ctx); err != nil {
			errChan <- fmt.Errorf("load generator failed: %w", err)
		}
	}()

	log.Printf("EventStore Load Generator started")
	log.Printf("Configuration: rate=%d req/s, initial_books=%d, scenario_weights=%v",
		cfg.Rate, cfg.InitialBooks, cfg.ScenarioWeights)
	log.Printf("Press Ctrl+C to stop...")

	// Wait for shutdown signal or error
	select {
	case sig := <-sigChan:
		log.Printf("Received signal %v, initiating graceful shutdown...", sig)
		cancel()
	case err := <-errChan:
		log.Printf("Error occurred: %v", err)
		cancel()
	}

	// Give some time for graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := loadGen.Stop(shutdownCtx); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	log.Printf("Load generator stopped")
}

func parseFlags() Config {
	var (
		rate              = flag.Int("rate", defaultRate, "Requests per second")
		observability     = flag.Bool("observability-enabled", false, "Enable OpenTelemetry observability")
		initialBooks      = flag.Int("initial-books", defaultInitialBooks, "Number of books to add initially")
		scenarioWeights   = flag.String("scenario-weights", defaultScenarioWeights, "Comma-separated weights for circulation,lending scenarios")
		stateSyncInterval = flag.Int("state-sync-interval", 60, "State synchronization interval in seconds")
	)

	flag.Parse()

	// Parse scenario weights
	weights, err := parseScenarioWeights(*scenarioWeights)
	if err != nil {
		log.Fatalf("Invalid scenario weights '%s': %v", *scenarioWeights, err)
	}

	return Config{
		Rate:                 *rate,
		ObservabilityEnabled: *observability,
		InitialBooks:         *initialBooks,
		ScenarioWeights:      weights,
		StateSyncIntervalSec: *stateSyncInterval,
	}
}

func parseScenarioWeights(weightsStr string) ([]int, error) {
	parts := strings.Split(weightsStr, ",")
	if len(parts) != 2 {
		return nil, fmt.Errorf("expected 2 weights, got %d", len(parts))
	}

	weights := make([]int, 2)
	total := 0
	for i, part := range parts {
		weight, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return nil, fmt.Errorf("invalid weight '%s': %w", part, err)
		}
		if weight < 0 || weight > 100 {
			return nil, fmt.Errorf("weight %d out of range [0, 100]", weight)
		}
		weights[i] = weight
		total += weight
	}

	if total != 100 {
		return nil, fmt.Errorf("weights must sum to 100, got %d", total)
	}

	return weights, nil
}

// ObservabilityConfig holds the observability adapters for the EventStore.
type ObservabilityConfig struct {
	Logger           eventstore.Logger
	ContextualLogger eventstore.ContextualLogger
	MetricsCollector eventstore.MetricsCollector
	TracingCollector eventstore.TracingCollector
}

func (c Config) NewObservabilityConfig() ObservabilityConfig {
	if !c.ObservabilityEnabled {
		return ObservabilityConfig{}
	}

	// Create real OpenTelemetry providers for the load generator
	_, err := config.NewTestObservabilityConfig()
	if err != nil {
		log.Printf("Failed to create observability providers: %v", err)
		return ObservabilityConfig{}
	}
	// Note: Providers are set globally in OpenTelemetry, no need to store reference

	// Create real OpenTelemetry adapters (same as the test)
	tracer := otel.Tracer("eventstore-load-generator")
	meter := otel.Meter("eventstore-load-generator")

	metricsCollector := oteladapters.NewMetricsCollector(meter)
	tracingCollector := oteladapters.NewTracingCollector(tracer)
	contextualLogger := oteladapters.NewSlogBridgeLogger("eventstore-load-generator")

	return ObservabilityConfig{
		Logger:           nil, // Using contextual logger instead
		ContextualLogger: contextualLogger,
		MetricsCollector: metricsCollector,
		TracingCollector: tracingCollector,
	}
}
