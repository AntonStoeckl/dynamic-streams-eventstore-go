// Package main implements a load generator for testing the Dynamic Event Streams EventStore
// with configurable request rates and realistic library management scenarios.
package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"runtime"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/A
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/addbookcopy"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/lendbookcopytoreader"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/removebookcopy"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
)

// ObservabilityConfig holds the observability adapters for command handlers.
type ObservabilityConfig struct {
	Logger           eventstore.Logger
	ContextualLogger eventstore.ContextualLogger
	MetricsCollector eventstore.MetricsCollector
	TracingCollector eventstore.TracingCollector
}

// LoadGenerator orchestrates realistic load generation against the EventStore
// with configurable request rates and library management scenarios.
type LoadGenerator struct {
	eventStore *postgresengine.EventStore
	config     Config

	// Command handlers for proper domain operations
	addBookCopyHandler    addbookcopy.CommandHandler
	lendBookCopyHandler   lendbookcopytoreader.CommandHandler
	returnBookCopyHandler returnbookcopyfromreader.CommandHandler
	removeBookCopyHandler removebookcopy.CommandHandler

	// Rate limiting
	ticker   *time.Ticker
	stopChan chan struct{}
	wg       sync.WaitGroup

	// Metrics and state
	requestCount int64
	errorCount   int64
	startTime    time.Time
	mu           sync.RWMutex

	// Note: EventStore will still collect its own metrics when observability is enabled
}

// NewLoadGenerator creates a new LoadGenerator instance with the provided EventStore and configuration.
func NewLoadGenerator(eventStore *postgresengine.EventStore, config Config, obsConfig ObservabilityConfig) *LoadGenerator {
	return &LoadGenerator{
		eventStore: eventStore,
		config:     config,
		stopChan:   make(chan struct{}),

		// Initialize command handlers with observability
		addBookCopyHandler:    mustCreateCommandHandler(addbookcopy.NewCommandHandler(eventStore, buildAddBookCopyOptions(obsConfig)...)),
		lendBookCopyHandler:   mustCreateCommandHandler(lendbookcopytoreader.NewCommandHandler(eventStore, buildLendBookCopyOptions(obsConfig)...)),
		returnBookCopyHandler: mustCreateCommandHandler(returnbookcopyfromreader.NewCommandHandler(eventStore, buildReturnBookCopyOptions(obsConfig)...)),
		removeBookCopyHandler: mustCreateCommandHandler(removebookcopy.NewCommandHandler(eventStore, buildRemoveBookCopyOptions(obsConfig)...)),
	}
}

// buildAddBookCopyOptions creates observability options for AddBookCopy command handler.
func buildAddBookCopyOptions(obsConfig ObservabilityConfig) []addbookcopy.Option {
	var options []addbookcopy.Option
	if obsConfig.MetricsCollector != nil {
		options = append(options, addbookcopy.WithMetrics(obsConfig.MetricsCollector))
	}
	if obsConfig.TracingCollector != nil {
		options = append(options, addbookcopy.WithTracing(obsConfig.TracingCollector))
	}
	if obsConfig.ContextualLogger != nil {
		options = append(options, addbookcopy.WithContextualLogging(obsConfig.ContextualLogger))
	}
	if obsConfig.Logger != nil {
		options = append(options, addbookcopy.WithLogging(obsConfig.Logger))
	}
	return options
}

// buildLendBookCopyOptions creates observability options for LendBookCopy command handler.
func buildLendBookCopyOptions(obsConfig ObservabilityConfig) []lendbookcopytoreader.Option {
	var options []lendbookcopytoreader.Option
	if obsConfig.MetricsCollector != nil {
		options = append(options, lendbookcopytoreader.WithMetrics(obsConfig.MetricsCollector))
	}
	if obsConfig.TracingCollector != nil {
		options = append(options, lendbookcopytoreader.WithTracing(obsConfig.TracingCollector))
	}
	if obsConfig.ContextualLogger != nil {
		options = append(options, lendbookcopytoreader.WithContextualLogging(obsConfig.ContextualLogger))
	}
	if obsConfig.Logger != nil {
		options = append(options, lendbookcopytoreader.WithLogging(obsConfig.Logger))
	}
	return options
}

// buildReturnBookCopyOptions creates observability options for ReturnBookCopy command handler.
func buildReturnBookCopyOptions(obsConfig ObservabilityConfig) []returnbookcopyfromreader.Option {
	var options []returnbookcopyfromreader.Option
	if obsConfig.MetricsCollector != nil {
		options = append(options, returnbookcopyfromreader.WithMetrics(obsConfig.MetricsCollector))
	}
	if obsConfig.TracingCollector != nil {
		options = append(options, returnbookcopyfromreader.WithTracing(obsConfig.TracingCollector))
	}
	if obsConfig.ContextualLogger != nil {
		options = append(options, returnbookcopyfromreader.WithContextualLogging(obsConfig.ContextualLogger))
	}
	if obsConfig.Logger != nil {
		options = append(options, returnbookcopyfromreader.WithLogging(obsConfig.Logger))
	}
	return options
}

// buildRemoveBookCopyOptions creates observability options for RemoveBookCopy command handler.
func buildRemoveBookCopyOptions(obsConfig ObservabilityConfig) []removebookcopy.Option {
	var options []removebookcopy.Option
	if obsConfig.MetricsCollector != nil {
		options = append(options, removebookcopy.WithMetrics(obsConfig.MetricsCollector))
	}
	if obsConfig.TracingCollector != nil {
		options = append(options, removebookcopy.WithTracing(obsConfig.TracingCollector))
	}
	if obsConfig.ContextualLogger != nil {
		options = append(options, removebookcopy.WithContextualLogging(obsConfig.ContextualLogger))
	}
	if obsConfig.Logger != nil {
		options = append(options, removebookcopy.WithLogging(obsConfig.Logger))
	}
	return options
}

// Start begins load generation with the configured request rate.
// It runs until the context is cancelled or Stop() is called.
func (lg *LoadGenerator) Start(ctx context.Context) error {
	lg.mu.Lock()
	lg.startTime = time.Now()
	lg.requestCount = 0
	lg.errorCount = 0
	lg.mu.Unlock()

	// Calculate an interval between requests based on the target rate
	interval := time.Second / time.Duration(lg.config.Rate)
	lg.ticker = time.NewTicker(interval)
	defer lg.ticker.Stop()

	log.Printf("Load generator starting with %d requests/second (interval: %v), initial goroutines: %d", lg.config.Rate, interval, runtime.NumGoroutine())

	// Start metrics reporting goroutine
	lg.wg.Add(1)
	go lg.metricsReporter(ctx)

	// Main load generation loop
	for {
		select {
		case <-ctx.Done():
			log.Printf("Load generator stopping due to context cancellation")
			return ctx.Err()

		case <-lg.stopChan:
			log.Printf("Load generator stopping due to stop signal")
			return nil

		case <-lg.ticker.C:
			lg.wg.Add(1)
			go lg.executeScenario(ctx)
		}
	}
}

// Stop gracefully shuts down the load generator.
func (lg *LoadGenerator) Stop(ctx context.Context) error {
	close(lg.stopChan)

	// Wait for all goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		lg.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		lg.logFinalStats()
		return nil
	case <-ctx.Done():
		lg.logFinalStats()
		return fmt.Errorf("shutdown timeout exceeded")
	}
}

// executeScenario runs a single load generation scenario based on configured weights.
func (lg *LoadGenerator) executeScenario(ctx context.Context) {
	defer lg.wg.Done()

	// Select a scenario based on weights (circulation: 4%, lending: 94%, errors: 2%)
	scenarioType := lg.selectScenario()

	var err error
	switch scenarioType {
	case "circulation":
		err = lg.runCirculationScenario(ctx)
	case "lending":
		err = lg.runLendingScenario(ctx)
	default:
		err = fmt.Errorf("unknown scenario type: %s", scenarioType)
	}

	// Note: EventStore operations are automatically instrumented when observability is enabled

	// Update internal counters
	lg.mu.Lock()
	lg.requestCount++
	if err != nil {
		lg.errorCount++
		log.Printf("Scenario error (%s): %v", scenarioType, err)
	}
	lg.mu.Unlock()
}

// selectScenario chooses a scenario type based on configured weights.
func (lg *LoadGenerator) selectScenario() string {
	// Generate random number 0-99
	r := rand.Intn(100) //nolint:gosec // Test code - weak random is acceptable

	// Apply weights: [circulation, lending]
	// Example: [20, 80] -> circulation: 0-19, lending: 20-99
	if r < lg.config.ScenarioWeights[0] {
		return "circulation"
	}

	return "lending"
}

// runCirculationScenario executes book circulation management operations using proper command handlers.
func (lg *LoadGenerator) runCirculationScenario(ctx context.Context) error {
	// Create timeout context for this operation (like benchmark tests)
	opCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	bookID := lg.generateRandomBookID()

	// Randomly choose between add and remove operations
	if rand.Intn(2) == 0 { //nolint:gosec // Test code - weak random is acceptable
		// Add a book to circulation
		command := addbookcopy.BuildCommand(
			bookID,
			"978-0000000000", // placeholder ISBN
			"Load Test Book",
			"Test Author",
			"1st Edition",
			"Test Publisher",
			2024,
			time.Now(),
		)

		return lg.addBookCopyHandler.Handle(opCtx, command)
	}

	// Remove a book from circulation
	command := removebookcopy.BuildCommand(bookID, time.Now())
	return lg.removeBookCopyHandler.Handle(opCtx, command)
}

// runLendingScenario executes book lending and return operations using proper command handlers.
func (lg *LoadGenerator) runLendingScenario(ctx context.Context) error {
	// Create timeout context for this operation (like benchmark tests)
	opCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	bookID := lg.generateRandomBookID()
	readerID := lg.generateRandomReaderID()

	// Randomly choose between lend and return operations
	if rand.Intn(2) == 0 { //nolint:gosec // Test code - weak random is acceptable
		// Lend a book to a reader
		command := lendbookcopytoreader.BuildCommand(bookID, readerID, time.Now())

		return lg.lendBookCopyHandler.Handle(opCtx, command)
	}
	// Return a book from a reader
	command := returnbookcopyfromreader.BuildCommand(bookID, readerID, time.Now())
	return lg.returnBookCopyHandler.Handle(opCtx, command)
}

// generateRandomBookID creates a random book ID for testing.
func (lg *LoadGenerator) generateRandomBookID() uuid.UUID {
	// Create deterministic UUIDs based on incremental numbers for better testing
	bookNum := rand.Int63n(1000) + 1 //nolint:gosec // Test code - weak random is acceptable
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("book-%d", bookNum)))
}

// generateRandomReaderID creates a random reader ID for testing.
func (lg *LoadGenerator) generateRandomReaderID() uuid.UUID {
	readerNum := rand.Int63n(100) + 1 //nolint:gosec // Test code - weak random is acceptable
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("reader-%d", readerNum)))
}

// metricsReporter logs metrics periodically.
func (lg *LoadGenerator) metricsReporter(ctx context.Context) {
	defer lg.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-lg.stopChan:
			return
		case <-ticker.C:
			lg.logCurrentStats()
		}
	}
}

// logCurrentStats logs current performance statistics.
func (lg *LoadGenerator) logCurrentStats() {
	lg.mu.RLock()
	duration := time.Since(lg.startTime)
	requests := lg.requestCount
	errors := lg.errorCount
	lg.mu.RUnlock()

	goroutineCount := runtime.NumGoroutine()

	if duration > 0 {
		rps := float64(requests) / duration.Seconds()
		errorRate := float64(errors) / float64(requests) * 100
		log.Printf("Stats: %d requests in %v (%.1f req/s), %d errors (%.1f%%), %d goroutines",
			requests, duration.Truncate(time.Second), rps, errors, errorRate, goroutineCount)
	}
}

// logFinalStats logs final performance statistics.
func (lg *LoadGenerator) logFinalStats() {
	lg.mu.RLock()
	duration := time.Since(lg.startTime)
	requests := lg.requestCount
	errors := lg.errorCount
	lg.mu.RUnlock()

	goroutineCount := runtime.NumGoroutine()

	if duration > 0 {
		rps := float64(requests) / duration.Seconds()
		errorRate := float64(errors) / float64(requests) * 100
		log.Printf("Final Stats: %d requests in %v (%.1f req/s), %d errors (%.1f%%), %d goroutines",
			requests, duration.Truncate(time.Second), rps, errors, errorRate, goroutineCount)
	}
}

// Note: All load generator metrics collection removed to simplify dashboard focus on EventStore metrics.
// EventStore operations are automatically instrumented when observability is enabled.

// mustCreateCommandHandler is a helper function that panics if command handler creation fails.
// This is appropriate for the load generator since it cannot continue without command handlers.
func mustCreateCommandHandler[T any](handler T, err error) T {
	if err != nil {
		panic(fmt.Sprintf("Failed to create command handler: %v", err))
	}
	return handler
}
