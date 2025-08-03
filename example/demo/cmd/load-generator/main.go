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

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell/config"
)

const (
	defaultRate            = 30
	defaultDatabaseURL     = "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
	defaultInitialBooks    = 1000
	defaultScenarioWeights = "40,50,10" // circulation, lending, errors
)

type Config struct {
	Rate                  int
	DatabaseURL           string
	ObservabilityEnabled  bool
	InitialBooks          int
	ScenarioWeights       []int
	StateSyncIntervalSec  int
}

func main() {
	config := parseFlags()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Initialize database connection
	pgxPool, err := pgxpool.New(ctx, config.DatabaseURL)
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
	if config.ObservabilityEnabled {
		obsConfig := config.NewObservabilityConfig()
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

	// Initialize and start load generator
	loadGen := NewLoadGenerator(eventStore, config)

	// Start load generation in a goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := loadGen.Start(ctx); err != nil {
			errChan <- fmt.Errorf("load generator failed: %w", err)
		}
	}()

	log.Printf("EventStore Load Generator started")
	log.Printf("Configuration: rate=%d req/s, initial_books=%d, scenario_weights=%v",
		config.Rate, config.InitialBooks, config.ScenarioWeights)
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
		databaseURL       = flag.String("database-url", defaultDatabaseURL, "PostgreSQL connection string")
		observability     = flag.Bool("observability-enabled", false, "Enable OpenTelemetry observability")
		initialBooks      = flag.Int("initial-books", defaultInitialBooks, "Number of books to add initially")
		scenarioWeights   = flag.String("scenario-weights", defaultScenarioWeights, "Comma-separated weights for circulation,lending,errors scenarios")
		stateSyncInterval = flag.Int("state-sync-interval", 60, "State synchronization interval in seconds")
	)

	flag.Parse()

	// Parse scenario weights
	weights, err := parseScenarioWeights(*scenarioWeights)
	if err != nil {
		log.Fatalf("Invalid scenario weights '%s': %v", *scenarioWeights, err)
	}

	return Config{
		Rate:                  *rate,
		DatabaseURL:           *databaseURL,
		ObservabilityEnabled:  *observability,
		InitialBooks:          *initialBooks,
		ScenarioWeights:       weights,
		StateSyncIntervalSec:  *stateSyncInterval,
	}
}

func parseScenarioWeights(weightsStr string) ([]int, error) {
	parts := strings.Split(weightsStr, ",")
	if len(parts) != 3 {
		return nil, fmt.Errorf("expected 3 weights, got %d", len(parts))
	}

	weights := make([]int, 3)
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

func (c Config) NewObservabilityConfig() config.ObservabilityConfig {
	if !c.ObservabilityEnabled {
		return config.ObservabilityConfig{}
	}
	return config.NewObservabilityConfig("eventstore-load-generator")
}