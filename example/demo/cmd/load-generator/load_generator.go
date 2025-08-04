package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/addbookcopy"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/lendbookcopytoreader"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/removebookcopy"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/returnbookcopyfromreader"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell"
)

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

	// OpenTelemetry metrics collection (optional)
	metricsCollector eventstore.MetricsCollector
}

// NewLoadGenerator creates a new LoadGenerator instance with the provided EventStore and configuration.
func NewLoadGenerator(eventStore *postgresengine.EventStore, config Config, metricsCollector eventstore.MetricsCollector) *LoadGenerator {
	return &LoadGenerator{
		eventStore:       eventStore,
		config:           config,
		stopChan:         make(chan struct{}),
		metricsCollector: metricsCollector,

		// Initialize command handlers
		addBookCopyHandler:    addbookcopy.NewCommandHandler(eventStore),
		lendBookCopyHandler:   lendbookcopytoreader.NewCommandHandler(eventStore),
		returnBookCopyHandler: returnbookcopyfromreader.NewCommandHandler(eventStore),
		removeBookCopyHandler: removebookcopy.NewCommandHandler(eventStore),
	}
}

// Start begins load generation with the configured request rate.
// It runs until the context is cancelled or Stop() is called.
func (lg *LoadGenerator) Start(ctx context.Context) error {
	lg.mu.Lock()
	lg.startTime = time.Now()
	lg.requestCount = 0
	lg.errorCount = 0
	lg.mu.Unlock()

	// Calculate interval between requests based on target rate
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

	// Select scenario based on weights (circulation: 4%, lending: 94%, errors: 2%)
	scenarioType := lg.selectScenario()

	// Record scenario execution start time for duration metrics
	startTime := time.Now()

	var err error
	switch scenarioType {
	case "circulation":
		err = lg.runCirculationScenario()
	case "lending":
		err = lg.runLendingScenario()
	default:
		err = fmt.Errorf("unknown scenario type: %s", scenarioType)
	}

	// Record metrics
	duration := time.Since(startTime)
	lg.recordMetrics(scenarioType, duration, err)

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
	r := rand.Intn(100)

	// Apply weights: [circulation, lending]
	// Example: [20, 80] -> circulation: 0-19, lending: 20-99
	if r < lg.config.ScenarioWeights[0] {
		return "circulation"
	} else {
		return "lending"
	}
}

// runCirculationScenario executes book circulation management operations using proper command handlers.
func (lg *LoadGenerator) runCirculationScenario() error {
	// Create timeout context for this operation (like benchmark tests)
	opCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	bookID := lg.generateRandomBookID()
	// Create null timing collector (don't collect timing for load generator)
	timingCollector := shell.NewTimingCollector(nil, nil, nil, nil)

	// Randomly choose between add and remove operations
	if rand.Intn(2) == 0 {
		// Add book to circulation
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

		return lg.addBookCopyHandler.Handle(opCtx, command, timingCollector)
	} else {
		// Remove book from circulation
		command := removebookcopy.BuildCommand(bookID, time.Now())

		return lg.removeBookCopyHandler.Handle(opCtx, command, timingCollector)
	}
}

// runLendingScenario executes book lending and return operations using proper command handlers.
func (lg *LoadGenerator) runLendingScenario() error {
	// Create timeout context for this operation (like benchmark tests)
	opCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	bookID := lg.generateRandomBookID()
	readerID := lg.generateRandomReaderID()
	// Create null timing collector (don't collect timing for load generator)
	timingCollector := shell.NewTimingCollector(nil, nil, nil, nil)

	// Randomly choose between lend and return operations
	if rand.Intn(2) == 0 {
		// Lend book to reader
		command := lendbookcopytoreader.BuildCommand(bookID, readerID, time.Now())

		return lg.lendBookCopyHandler.Handle(opCtx, command, timingCollector)
	} else {
		// Return book from reader
		command := returnbookcopyfromreader.BuildCommand(bookID, readerID, time.Now())

		return lg.returnBookCopyHandler.Handle(opCtx, command, timingCollector)
	}
}

// generateRandomBookID creates a random book ID for testing.
func (lg *LoadGenerator) generateRandomBookID() uuid.UUID {
	// Create deterministic UUIDs based on incremental numbers for better testing
	bookNum := rand.Int63n(1000) + 1
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("book-%d", bookNum)))
}

// generateRandomReaderID creates a random reader ID for testing.
func (lg *LoadGenerator) generateRandomReaderID() uuid.UUID {
	readerNum := rand.Int63n(100) + 1
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

// recordMetrics records OpenTelemetry metrics for load generator operations.
func (lg *LoadGenerator) recordMetrics(scenarioType string, duration time.Duration, err error) {
	if lg.metricsCollector == nil {
		return // Metrics collection is optional
	}

	labels := map[string]string{
		"scenario_type": scenarioType,
		"service":       "load-generator",
	}

	// Record request duration
	lg.metricsCollector.RecordDuration("load_generator_request_duration_seconds", duration, labels)

	// Record scenario counters
	lg.metricsCollector.IncrementCounter("load_generator_scenarios_total", labels)

	// Record errors if any
	if err != nil {
		errorLabels := map[string]string{
			"scenario_type": scenarioType,
			"service":       "load-generator",
			"error_type":    lg.classifyError(err),
		}
		lg.metricsCollector.IncrementCounter("load_generator_errors_total", errorLabels)
	}

	// Record current request rate (calculated every 10 requests for efficiency)
	lg.mu.RLock()
	if lg.requestCount%10 == 0 && lg.requestCount > 0 {
		currentDuration := time.Since(lg.startTime)
		if currentDuration > 0 {
			currentRate := float64(lg.requestCount) / currentDuration.Seconds()
			lg.metricsCollector.RecordValue("load_generator_current_rate", currentRate, map[string]string{"service": "load-generator"})
		}
	}
	lg.mu.RUnlock()
}

// classifyError categorizes errors for better metrics labeling.
func (lg *LoadGenerator) classifyError(err error) string {
	errStr := err.Error()
	switch {
	case strings.Contains(errStr, "concurrency error"):
		return "concurrency_conflict"
	case strings.Contains(errStr, "context canceled"):
		return "context_canceled"
	case strings.Contains(errStr, "query failed"):
		return "query_error"
	case strings.Contains(errStr, "append failed"):
		return "append_error"
	case strings.Contains(errStr, "marshaling failed"):
		return "marshaling_error"
	case strings.Contains(errStr, "unmarshaling failed"):
		return "unmarshaling_error"
	default:
		return "unknown_error"
	}
}
