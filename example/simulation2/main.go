package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("Simulation failed: %v", err)
	}
}

func run() error {
	log.Printf("%s %s", Success("🎭"), Success("Starting Actor-Based Library Simulation v2"))

	cfg := parseFlags()

	logDBAdapter()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Initialize EventStore (start with PGX, expand later).
	eventStore, err := initializePGXEventStore(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to create EventStore: %w", err)
	}

	logSimulationConfiguration(cfg)

	// Create simulation context BEFORE initialization
	simulationCtx, simulationCancel := context.WithCancel(ctx)
	defer simulationCancel()

	// Initialize simulation.
	_, simulation, err := initializeSimulation(simulationCtx, eventStore, cfg)
	if err != nil {
		return err
	}

	logStartup()

	// Start simulation in the goroutine so we can handle signals
	simulationDone := make(chan error, 1)
	go func() {
		simulationDone <- simulation.Start(cfg)
	}()

	// Wait for a shutdown signal or simulation completion.
	select {
	case sig := <-sigChan:
		log.Printf("📢 Received signal %v, initiating graceful shutdown...", sig)
		simulationCancel()
		// Wait for simulation to finish
		select {
		case err := <-simulationDone:
			if err != nil {
				log.Printf("⚠️  Simulation ended with error: %v", err)
			} else {
				log.Printf("📢 Simulation goroutine finished cleanly")
			}
		case <-time.After(5 * time.Second):
			log.Printf("⚠️  Simulation shutdown timeout after 5 seconds")
		}
	case <-simulationCtx.Done():
		log.Printf("📢 Simulation context canceled")
	case err := <-simulationDone:
		if err != nil {
			return fmt.Errorf("simulation failed: %w", err)
		}
	}

	// Graceful shutdown.
	gracefulShutdown(simulation)

	return nil
}

func logDBAdapter() {
	// Database adapter configuration.
	adapterType := os.Getenv("DB_ADAPTER")
	if adapterType == "" {
		adapterType = "pgx"
	}
	log.Printf("%s Using database adapter: %s", Info("🔧"), Info(adapterType))
}

func logSimulationConfiguration(cfg Config) {
	log.Printf("📊 Simulation Configuration:")
	log.Printf("  - Active Readers: %d (fixed)",
		cfg.ActiveReaders)
	log.Printf("  - Population: %d-%d readers, %d-%d books",
		MinReaders, MaxReaders, MinBooks, MaxBooks)
	log.Printf("  - Concurrent Workers: %d", cfg.Workers)
	log.Printf("  - Librarian Staff: %d", cfg.LibrarianCount)
}

func initializeSimulation(ctx context.Context, eventStore *postgresengine.EventStore, cfg Config) (*HandlerBundle, *Simulation, error) {
	log.Printf("🏗️  Initializing simulation...")

	// Create handlers for all library operations.
	handlers, err := NewHandlerBundle(eventStore, cfg) //nolint:contextcheck // Initialization code, context created internally
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create handler bundle: %w", err)
	}

	// Create the simulation state for fast lookups.
	state := NewSimulationState()

	// Create the simulation.
	simulation, err := NewSimulation(ctx, eventStore, state, handlers, cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create simulation: %w", err)
	}

	// Connect simulation state to handlers for actor decisions.
	handlers.SetSimulationState(state)

	// NOTE: No need for load controller or async metrics - simulation handles everything directly

	return handlers, simulation, nil
}

func logStartup() {
	log.Printf("⏳ Simulation will start in %d seconds...", SetupPhaseDelaySeconds)
	time.Sleep(time.Duration(SetupPhaseDelaySeconds) * time.Second)

	log.Printf("🚀 Simulation starting...")
	log.Printf("📊 Performance measurement with rolling window enabled")
	log.Printf("Press Ctrl+C to stop...")
}

func gracefulShutdown(simulation *Simulation) {
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	log.Printf("🔄 Shutting down simulation...")

	// Stop the simulation.
	if err := simulation.Stop(); err != nil {
		log.Printf("⚠️  Error stopping simulation: %v", err)
	}

	select {
	case <-shutdownCtx.Done():
		log.Printf("⚠️  Shutdown timeout exceeded")
	default:
		log.Printf("✅ Library Simulation stopped gracefully")
	}
}
