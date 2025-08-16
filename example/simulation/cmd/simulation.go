package main

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/addbookcopy"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/cancelreadercontract"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/lendbookcopytoreader"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/registerreader"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/removebookcopy"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/returnbookcopyfromreader"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/query/booksincirculation"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/query/bookslentbyreader"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/query/bookslentout"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/query/registeredreaders"
)

// ObservabilityConfig holds the observability adapters for command handlers.
type ObservabilityConfig struct {
	Logger           eventstore.Logger
	ContextualLogger eventstore.ContextualLogger
	MetricsCollector eventstore.MetricsCollector
	TracingCollector eventstore.TracingCollector
}

// Request represents a single simulation request to be processed by workers.
type Request struct {
	ctx        context.Context
	scenario   Scenario
	resultChan chan error
}

// LibrarySimulation orchestrates realistic library management simulation against the EventStore
// using a worker pool pattern with state-aware scenario generation.
type LibrarySimulation struct {
	eventStore *postgresengine.EventStore
	config     Config
	state      *SimulationState
	selector   *ScenarioSelector

	// Command handlers for library operations
	addBookCopyHandler    addbookcopy.CommandHandler
	removeBookCopyHandler removebookcopy.CommandHandler
	registerReaderHandler registerreader.CommandHandler
	cancelReaderHandler   cancelreadercontract.CommandHandler
	lendBookCopyHandler   lendbookcopytoreader.CommandHandler
	returnBookCopyHandler returnbookcopyfromreader.CommandHandler

	// Query handlers for state initialization and occasional queries
	booksInCirculationHandler booksincirculation.QueryHandler
	booksLentOutHandler       bookslentout.QueryHandler
	booksLentByReaderHandler  bookslentbyreader.QueryHandler
	registeredReadersHandler  registeredreaders.QueryHandler

	// Worker pool architecture (copied from load generator)
	requestQueue chan *Request
	workerCount  int
	stopChan     chan struct{}
	wg           sync.WaitGroup

	// Metrics and state
	requestCount      int64
	errorCount        int64
	backpressureCount int64
	startTime         time.Time
	mu                sync.RWMutex

	// Profiling callback (called after setup phase)
	profilingCallback func()
}

// NewLibrarySimulation creates a new LibrarySimulation instance with the provided EventStore and configuration.
func NewLibrarySimulation(eventStore *postgresengine.EventStore, config Config, obsConfig ObservabilityConfig) (*LibrarySimulation, error) {
	// Calculate worker count based on connection pool capacity (avoid oversubscription)
	workerCount := 85            // Optimized for ~75 req/sec sustained performance
	queueSize := workerCount * 2 // Bounded queue to prevent memory leaks

	state := NewSimulationState()
	selector := NewScenarioSelector(state, config)

	// Create all handlers using helper methods
	handlers, err := createHandlers(eventStore, obsConfig)
	if err != nil {
		return nil, err
	}

	return &LibrarySimulation{
		eventStore: eventStore,
		config:     config,
		state:      state,
		selector:   selector,
		stopChan:   make(chan struct{}),

		// Worker pool configuration
		requestQueue: make(chan *Request, queueSize),
		workerCount:  workerCount,

		// Initialize command handlers
		addBookCopyHandler:    handlers.addBookCopyHandler,
		removeBookCopyHandler: handlers.removeBookCopyHandler,
		registerReaderHandler: handlers.registerReaderHandler,
		cancelReaderHandler:   handlers.cancelReaderHandler,
		lendBookCopyHandler:   handlers.lendBookCopyHandler,
		returnBookCopyHandler: handlers.returnBookCopyHandler,

		// Initialize query handlers
		booksInCirculationHandler: handlers.booksInCirculationHandler,
		booksLentOutHandler:       handlers.booksLentOutHandler,
		booksLentByReaderHandler:  handlers.booksLentByReaderHandler,
		registeredReadersHandler:  handlers.registeredReadersHandler,
	}, nil
}

// SetProfilingCallback sets a callback function to be called after the setup phase
// to enable delayed profiling start.
func (ls *LibrarySimulation) SetProfilingCallback(callback func()) {
	ls.profilingCallback = callback
}

// handlerBundle holds all command and query handlers for the simulation.
type handlerBundle struct {
	// Command handlers
	addBookCopyHandler    addbookcopy.CommandHandler
	removeBookCopyHandler removebookcopy.CommandHandler
	registerReaderHandler registerreader.CommandHandler
	cancelReaderHandler   cancelreadercontract.CommandHandler
	lendBookCopyHandler   lendbookcopytoreader.CommandHandler
	returnBookCopyHandler returnbookcopyfromreader.CommandHandler

	// Query handlers
	booksInCirculationHandler booksincirculation.QueryHandler
	booksLentOutHandler       bookslentout.QueryHandler
	booksLentByReaderHandler  bookslentbyreader.QueryHandler
	registeredReadersHandler  registeredreaders.QueryHandler
}

// createHandlers creates all command and query handlers with observability options.
//
//nolint:funlen // Repetitive handler creation with consistent error handling
func createHandlers(eventStore *postgresengine.EventStore, obsConfig ObservabilityConfig) (*handlerBundle, error) {
	// Create command handlers
	addBookCopyHandler, err := addbookcopy.NewCommandHandler(eventStore, buildAddBookCopyOptions(obsConfig)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create AddBookCopy handler: %w", err)
	}

	removeBookCopyHandler, err := removebookcopy.NewCommandHandler(eventStore, buildRemoveBookCopyOptions(obsConfig)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create RemoveBookCopy handler: %w", err)
	}

	registerReaderHandler, err := registerreader.NewCommandHandler(eventStore, buildRegisterReaderOptions(obsConfig)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create RegisterReader handler: %w", err)
	}

	cancelReaderHandler, err := cancelreadercontract.NewCommandHandler(eventStore, buildCancelReaderContractOptions(obsConfig)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create CancelReaderContract handler: %w", err)
	}

	lendBookCopyHandler, err := lendbookcopytoreader.NewCommandHandler(eventStore, buildLendBookCopyOptions(obsConfig)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create LendBookCopy handler: %w", err)
	}

	returnBookCopyHandler, err := returnbookcopyfromreader.NewCommandHandler(eventStore, buildReturnBookCopyOptions(obsConfig)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create ReturnBookCopy handler: %w", err)
	}

	// Create query handlers
	booksInCirculationHandler, err := booksincirculation.NewQueryHandler(eventStore, buildBooksInCirculationOptions(obsConfig)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create BooksInCirculation handler: %w", err)
	}

	booksLentOutHandler, err := bookslentout.NewQueryHandler(eventStore, buildBooksLentOutOptions(obsConfig)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create BooksLentOut handler: %w", err)
	}

	booksLentByReaderHandler, err := bookslentbyreader.NewQueryHandler(eventStore, buildBooksLentByReaderOptions(obsConfig)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create BooksLentByReader handler: %w", err)
	}

	registeredReadersHandler, err := registeredreaders.NewQueryHandler(eventStore, buildRegisteredReadersOptions(obsConfig)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create RegisteredReaders handler: %w", err)
	}

	return &handlerBundle{
		addBookCopyHandler:        addBookCopyHandler,
		removeBookCopyHandler:     removeBookCopyHandler,
		registerReaderHandler:     registerReaderHandler,
		cancelReaderHandler:       cancelReaderHandler,
		lendBookCopyHandler:       lendBookCopyHandler,
		returnBookCopyHandler:     returnBookCopyHandler,
		booksInCirculationHandler: booksInCirculationHandler,
		booksLentOutHandler:       booksLentOutHandler,
		booksLentByReaderHandler:  booksLentByReaderHandler,
		registeredReadersHandler:  registeredReadersHandler,
	}, nil
}

// Start begins the simulation with startup state initialization followed by main simulation.
func (ls *LibrarySimulation) Start(ctx context.Context) error {
	log.Printf("Library Simulation starting with setup phase...")

	// Phase 1: Initialize simulation state from existing events
	if err := ls.initializeState(ctx); err != nil {
		return fmt.Errorf("failed to initialize state: %w", err)
	}

	// Phase 2: Add books/readers to reach target numbers
	setupPerformed, err := ls.setupInitialState(ctx)
	if err != nil {
		return fmt.Errorf("failed to setup initial state: %w", err)
	}

	// Phase 3: Sleep for Grafana visibility ONLY if setup was performed
	if setupPerformed {
		log.Printf("Setup phase complete. Sleeping for %d seconds for Grafana visibility...", ls.config.SetupSleepSeconds)
		time.Sleep(time.Duration(ls.config.SetupSleepSeconds) * time.Second)
	}

	// Phase 4: Start profiling if callback is set (after setup phase)
	if ls.profilingCallback != nil {
		ls.profilingCallback()
	}

	// Phase 5: Start main simulation
	log.Printf("Starting main simulation phase...")
	return ls.runMainSimulation(ctx)
}

// Stop gracefully shuts down the simulation.
func (ls *LibrarySimulation) Stop(ctx context.Context) error {
	close(ls.stopChan) // Signal main simulation loop to stop

	// Main simulation loop will handle closing requestQueue and waiting for workers
	// We just wait for everything to finish with a timeout

	// Wait for all goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		ls.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		ls.logFinalStats()
		return nil
	case <-ctx.Done():
		ls.logFinalStats()
		return fmt.Errorf("shutdown timeout exceeded")
	}
}

// initializeState queries existing events to build initial simulation state.
func (ls *LibrarySimulation) initializeState(ctx context.Context) error {
	log.Printf("Querying existing state from EventStore...")

	// Query books in circulation
	booksResult, err := ls.booksInCirculationHandler.Handle(ctx)
	if err != nil {
		return fmt.Errorf("failed to query books in circulation: %w", err)
	}

	for _, book := range booksResult.Books {
		bookID, err := uuid.Parse(book.BookID)
		if err != nil {
			log.Printf("Warning: invalid book ID '%s': %v", book.BookID, err)
			continue
		}
		ls.state.AddBook(bookID)
	}

	// Query registered readers
	readersResult, err := ls.registeredReadersHandler.Handle(ctx)
	if err != nil {
		return fmt.Errorf("failed to query registered readers: %w", err)
	}

	for _, reader := range readersResult.Readers {
		readerID, err := uuid.Parse(reader.ReaderID)
		if err != nil {
			log.Printf("Warning: invalid reader ID '%s': %v", reader.ReaderID, err)
			continue
		}
		ls.state.AddReader(readerID)
	}

	// Query books lent out to build lending relationships
	lentBooksResult, err := ls.booksLentOutHandler.Handle(ctx)
	if err != nil {
		return fmt.Errorf("failed to query books lent out: %w", err)
	}

	for _, lentBook := range lentBooksResult.Lendings {
		bookID, err := uuid.Parse(lentBook.BookID)
		if err != nil {
			log.Printf("Warning: invalid book ID '%s': %v", lentBook.BookID, err)
			continue
		}
		readerID, err := uuid.Parse(lentBook.ReaderID)
		if err != nil {
			log.Printf("Warning: invalid reader ID '%s': %v", lentBook.ReaderID, err)
			continue
		}
		ls.state.LendBook(bookID, readerID)
	}

	books, readers, lending := ls.state.GetStats()
	log.Printf("Initial state: %d books, %d readers, %d lending relationships", books, readers, lending)

	return nil
}

// setupInitialState adds books and readers to reach target numbers using concurrent worker pool.
// Returns true if any setup operations were performed (requiring a pause for Grafana visibility).
func (ls *LibrarySimulation) setupInitialState(ctx context.Context) (bool, error) {
	books, readers, _ := ls.state.GetStats()

	booksNeeded := ls.config.MinBooks - books
	readersNeeded := ls.config.MinReaders - readers
	totalSetupOperations := booksNeeded + readersNeeded

	if totalSetupOperations <= 0 {
		log.Printf("No setup needed: %d books, %d readers already exist", books, readers)
		return false, nil
	}

	log.Printf("Setup phase: adding %d books + %d readers = %d total operations using worker pool",
		booksNeeded, readersNeeded, totalSetupOperations)

	// Start workers for setup
	ls.startWorkers(ctx)
	defer ls.stopWorkers()

	// Create setup scenarios and queue them
	setupErrors := 0
	var wg sync.WaitGroup

	// Queue book creation scenarios
	for i := 0; i < booksNeeded; i++ {
		bookScenario := Scenario{
			Type:   ScenarioAddBook,
			BookID: uuid.New(),
		}

		wg.Add(1)
		ls.queueSetupRequest(ctx, bookScenario, &wg, &setupErrors)
	}

	// Queue reader registration scenarios
	for i := 0; i < readersNeeded; i++ {
		readerScenario := Scenario{
			Type:     ScenarioRegisterReader,
			ReaderID: uuid.New(),
		}

		wg.Add(1)
		ls.queueSetupRequest(ctx, readerScenario, &wg, &setupErrors)
	}

	// Wait for all setup operations to complete
	wg.Wait()

	// Report results
	finalBooks, finalReaders, finalLending := ls.state.GetStats()
	if setupErrors > 0 {
		log.Printf("Setup completed with %d errors: %d books, %d readers, %d lending relationships",
			setupErrors, finalBooks, finalReaders, finalLending)
	} else {
		log.Printf("Setup completed successfully: %d books, %d readers, %d lending relationships",
			finalBooks, finalReaders, finalLending)
	}

	return true, nil
}

// startWorkers starts the worker pool for setup operations.
func (ls *LibrarySimulation) startWorkers(ctx context.Context) {
	// Start fixed number of worker goroutines for setup
	for i := 0; i < ls.workerCount; i++ {
		ls.wg.Add(1)
		go ls.worker(ctx, i)
	}
}

// stopWorkers gracefully stops the worker pool after setup.
func (ls *LibrarySimulation) stopWorkers() {
	// Close request queue to signal workers to stop
	close(ls.requestQueue)

	// Wait for all workers to finish
	ls.wg.Wait()

	// Recreate the request queue for main simulation
	ls.requestQueue = make(chan *Request, cap(ls.requestQueue))
}

// queueSetupRequest queues a single setup request through the worker pool.
func (ls *LibrarySimulation) queueSetupRequest(ctx context.Context, scenario Scenario, wg *sync.WaitGroup, errorCount *int) {
	request := &Request{
		ctx:        ctx,
		scenario:   scenario,
		resultChan: make(chan error, 1),
	}

	// Queue the request (blocking if queue is full)
	select {
	case ls.requestQueue <- request:
		// Successfully queued, wait for result in goroutine
		go func() {
			defer wg.Done()
			select {
			case err := <-request.resultChan:
				if err != nil {
					ls.mu.Lock()
					*errorCount++
					ls.mu.Unlock()
					log.Printf("Setup error (%s): %v", scenario.Type, err)
				}
			case <-ctx.Done():
				log.Printf("Setup canceled for scenario %s", scenario.Type)
			}
		}()
	case <-ctx.Done():
		wg.Done()
		log.Printf("Setup canceled before queueing scenario %s", scenario.Type)
	}
}

// runMainSimulation executes the main simulation loop with worker pool architecture.
func (ls *LibrarySimulation) runMainSimulation(ctx context.Context) error {
	ls.mu.Lock()
	ls.startTime = time.Now()
	ls.requestCount = 0
	ls.errorCount = 0
	ls.backpressureCount = 0
	ls.mu.Unlock()

	// Calculate batching parameters for higher rate precision
	// For rates >50 req/sec, batch multiple requests to avoid timer precision issues
	batchSize := 1
	batchInterval := time.Second / time.Duration(ls.config.Rate)

	if ls.config.Rate >= 50 {
		// Batch approach: generate multiple requests per ticker interval
		// This avoids OS timer precision limitations at high rates
		batchSize = ls.config.Rate / 10 // 10 batches per second
		if batchSize < 1 {
			batchSize = 1
		}
		batchInterval = 100 * time.Millisecond // 10 batches per second
	}

	ticker := time.NewTicker(batchInterval)
	defer ticker.Stop()

	log.Printf("Main simulation starting with %d requests/second (batch: %d req every %v), %d workers, goroutines: %d",
		ls.config.Rate, batchSize, batchInterval, ls.workerCount, runtime.NumGoroutine())

	// Start fixed number of worker goroutines
	for i := 0; i < ls.workerCount; i++ {
		ls.wg.Add(1)
		go ls.worker(ctx, i)
	}

	// Start metrics reporting goroutine
	ls.wg.Add(1)
	go ls.metricsReporter(ctx)

	// Rate-limited request generation loop with batching
	for {
		select {
		case <-ctx.Done():
			log.Printf("Main simulation stopping due to context cancellation - initiating graceful shutdown")
			close(ls.requestQueue) // Signal workers to stop accepting new requests
			ls.wg.Wait()           // Wait for all workers to finish current requests
			return ctx.Err()

		case <-ls.stopChan:
			log.Printf("Main simulation stopping due to stop signal - initiating graceful shutdown")
			close(ls.requestQueue) // Signal workers to stop accepting new requests
			ls.wg.Wait()           // Wait for all workers to finish current requests
			return nil

		case <-ticker.C:
			// Generate and queue multiple requests per batch for higher throughput
			for i := 0; i < batchSize; i++ {
				// Generate realistic scenario instead of random request
				scenario := ls.selector.SelectScenario()
				request := ls.generateRequest(ctx, scenario)

				// Non-blocking request submission (backpressure handling)
				select {
				case ls.requestQueue <- request:
					// Request queued successfully
				default:
					// Queue full - record backpressure and fail fast
					ls.recordBackpressure()
					close(request.resultChan) // Signal failure
				}
			}
		}
	}
}

// Additional helper functions for command handler options...
// (Similar to load generator but adapted for all command types)

// generateRequest creates a request structure for the worker pool.
func (ls *LibrarySimulation) generateRequest(ctx context.Context, scenario Scenario) *Request {
	return &Request{
		ctx:        ctx,
		scenario:   scenario,
		resultChan: make(chan error, 1),
	}
}

// recordBackpressure records backpressure events when queue is full.
func (ls *LibrarySimulation) recordBackpressure() {
	ls.mu.Lock()
	ls.backpressureCount++
	ls.mu.Unlock()
}

// worker processes requests from the queue with bounded concurrency.
func (ls *LibrarySimulation) worker(ctx context.Context, workerID int) {
	defer ls.wg.Done()

	log.Printf("Worker %d starting", workerID)

	for {
		select {
		case request, ok := <-ls.requestQueue:
			if !ok {
				log.Printf("Worker %d stopping - queue closed", workerID)
				return
			}

			// Execute request with faster timeout (1 second)
			err := ls.executeRequest(request) //nolint:contextcheck

			// Send result back safely - protect against closed channels and context cancellation
			select {
			case request.resultChan <- err:
				// Successfully sent result
			case <-ctx.Done():
				// Context cancelled during result sending
				return
			default:
				// Channel closed or blocked - don't panic, just log
				log.Printf("Worker %d: could not send result for %s (channel closed/blocked)", workerID, request.scenario.Type)
			}

			// Update counters and state
			ls.updateCountersAndState(workerID, request.scenario, err)

		case <-ctx.Done():
			log.Printf("Worker %d stopping due to context cancellation", workerID)
			return
		}
	}
}

// executeRequest executes a single request based on scenario type.
func (ls *LibrarySimulation) executeRequest(request *Request) error {
	// Create timeout context for this operation
	opCtx, cancel := context.WithTimeout(request.ctx, 1*time.Second)
	defer cancel()

	scenario := request.scenario

	// For return operations, try to reserve the book first
	if scenario.Type == ScenarioReturnBook {
		if !ls.state.ReserveReturn(scenario.BookID) {
			// Book is already being returned by another worker
			// This is not an error, just skip this operation
			return nil
		}
		// Reservation successful - it will be released in updateCountersAndState
	}

	switch scenario.Type {
	case ScenarioAddBook:
		return ls.executeAddBook(opCtx, scenario.BookID)
	case ScenarioRemoveBook:
		return ls.executeRemoveBook(opCtx, scenario.BookID)
	case ScenarioRegisterReader:
		return ls.executeRegisterReader(opCtx, scenario.ReaderID)
	case ScenarioCancelReader:
		return ls.executeCancelReaderContract(opCtx, scenario.ReaderID)
	case ScenarioLendBook:
		return ls.executeLendBook(opCtx, scenario.BookID, scenario.ReaderID)
	case ScenarioReturnBook:
		return ls.executeReturnBook(opCtx, scenario.BookID, scenario.ReaderID)
	case ScenarioQueryBooks:
		return ls.executeQueryBooks(opCtx, scenario.ReaderID)
	default:
		return fmt.Errorf("unknown scenario type: %s", scenario.Type)
	}
}

// updateCountersAndState updates metrics and simulation state after request execution.
func (ls *LibrarySimulation) updateCountersAndState(workerID int, scenario Scenario, err error) {
	// Always release return reservation if this was a return scenario
	// This must happen regardless of success/failure to prevent deadlocks
	if scenario.Type == ScenarioReturnBook {
		ls.state.ReleaseReturn(scenario.BookID)
	}

	ls.mu.Lock()
	ls.requestCount++
	if err != nil {
		ls.errorCount++
		if scenario.IsError {
			log.Printf("Worker %d expected error (%s - %s): %v", workerID, scenario.Type, scenario.Reason, err)
		} else {
			log.Printf("Worker %d unexpected error (%s): %v", workerID, scenario.Type, err)
		}
	} else {
		// Update simulation state on successful operations
		ls.updateSimulationState(scenario)
	}
	ls.mu.Unlock()
}

// updateSimulationState updates the in-memory simulation state after successful operations.
func (ls *LibrarySimulation) updateSimulationState(scenario Scenario) {
	switch scenario.Type {
	case ScenarioAddBook:
		ls.state.AddBook(scenario.BookID)
	case ScenarioRemoveBook:
		ls.state.RemoveBook(scenario.BookID)
	case ScenarioRegisterReader:
		ls.state.AddReader(scenario.ReaderID)
	case ScenarioCancelReader:
		ls.state.RemoveReader(scenario.ReaderID)
	case ScenarioLendBook:
		ls.state.LendBook(scenario.BookID, scenario.ReaderID)
	case ScenarioReturnBook:
		ls.state.ReturnBook(scenario.BookID, scenario.ReaderID)
		// Note: ScenarioQueryBooks doesn't change state
	}
}

// Individual execution methods for each scenario type...
func (ls *LibrarySimulation) executeAddBook(ctx context.Context, bookID uuid.UUID) error {
	command := addbookcopy.BuildCommand(
		bookID,
		"978-0000000000", // placeholder ISBN
		"Simulation Test Book",
		"Test Author",
		"1st Edition",
		"Test Publisher",
		2024,
		time.Now(),
	)
	return ls.addBookCopyHandler.Handle(ctx, command)
}

func (ls *LibrarySimulation) executeRemoveBook(ctx context.Context, bookID uuid.UUID) error {
	command := removebookcopy.BuildCommand(bookID, time.Now())
	return ls.removeBookCopyHandler.Handle(ctx, command)
}

func (ls *LibrarySimulation) executeRegisterReader(ctx context.Context, readerID uuid.UUID) error {
	command := registerreader.BuildCommand(readerID, "Test Reader", time.Now())
	return ls.registerReaderHandler.Handle(ctx, command)
}

func (ls *LibrarySimulation) executeCancelReaderContract(ctx context.Context, readerID uuid.UUID) error {
	command := cancelreadercontract.BuildCommand(readerID, time.Now())
	return ls.cancelReaderHandler.Handle(ctx, command)
}

func (ls *LibrarySimulation) executeLendBook(ctx context.Context, bookID, readerID uuid.UUID) error {
	command := lendbookcopytoreader.BuildCommand(bookID, readerID, time.Now())
	return ls.lendBookCopyHandler.Handle(ctx, command)
}

func (ls *LibrarySimulation) executeReturnBook(ctx context.Context, bookID, readerID uuid.UUID) error {
	command := returnbookcopyfromreader.BuildCommand(bookID, readerID, time.Now())
	return ls.returnBookCopyHandler.Handle(ctx, command)
}

func (ls *LibrarySimulation) executeQueryBooks(ctx context.Context, readerID uuid.UUID) error {
	query := bookslentbyreader.BuildQuery(readerID)
	_, err := ls.booksLentByReaderHandler.Handle(ctx, query)
	return err
}

// Metrics reporting and logging methods...
func (ls *LibrarySimulation) metricsReporter(ctx context.Context) {
	defer ls.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ls.stopChan:
			return
		case <-ticker.C:
			ls.logCurrentStats()
		}
	}
}

func (ls *LibrarySimulation) logCurrentStats() {
	ls.mu.RLock()
	duration := time.Since(ls.startTime)
	requests := ls.requestCount
	errors := ls.errorCount
	backpressure := ls.backpressureCount
	ls.mu.RUnlock()

	books, readers, activeLendings, completedLendings := ls.state.GetDetailedStats()
	goroutineCount := runtime.NumGoroutine()
	queueLength := len(ls.requestQueue)

	if duration > 0 {
		rps := float64(requests) / duration.Seconds()
		errorRate := float64(errors) / float64(requests) * 100
		backpressureRate := float64(backpressure) / float64(requests+backpressure) * 100

		log.Printf("Stats: %d req in %v (%.1f req/s), %d err (%.1f%%), %d backpressure (%.1f%%), queue: %d, %d goroutines",
			requests, duration.Truncate(time.Second), rps, errors, errorRate, backpressure, backpressureRate, queueLength, goroutineCount)
		log.Printf("State: %d books, %d readers, %d active lendings, %d completed lendings (total: %d)",
			books, readers, activeLendings, completedLendings, activeLendings+completedLendings)
	}
}

func (ls *LibrarySimulation) logFinalStats() {
	ls.mu.RLock()
	duration := time.Since(ls.startTime)
	requests := ls.requestCount
	errors := ls.errorCount
	backpressure := ls.backpressureCount
	ls.mu.RUnlock()

	books, readers, activeLendings, completedLendings := ls.state.GetDetailedStats()
	goroutineCount := runtime.NumGoroutine()

	if duration > 0 {
		rps := float64(requests) / duration.Seconds()
		errorRate := float64(errors) / float64(requests) * 100
		backpressureRate := float64(backpressure) / float64(requests+backpressure) * 100

		log.Printf("Final Stats: %d req in %v (%.1f req/s), %d err (%.1f%%), %d backpressure (%.1f%%), %d goroutines",
			requests, duration.Truncate(time.Second), rps, errors, errorRate, backpressure, backpressureRate, goroutineCount)
		log.Printf("Final State: %d books, %d readers, %d active lendings, %d completed lendings (total: %d)",
			books, readers, activeLendings, completedLendings, activeLendings+completedLendings)
	}
}

// Helper functions for building observability options (similar to load generator)...
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

func buildRegisterReaderOptions(obsConfig ObservabilityConfig) []registerreader.Option {
	var options []registerreader.Option
	if obsConfig.MetricsCollector != nil {
		options = append(options, registerreader.WithMetrics(obsConfig.MetricsCollector))
	}
	if obsConfig.TracingCollector != nil {
		options = append(options, registerreader.WithTracing(obsConfig.TracingCollector))
	}
	if obsConfig.ContextualLogger != nil {
		options = append(options, registerreader.WithContextualLogging(obsConfig.ContextualLogger))
	}
	if obsConfig.Logger != nil {
		options = append(options, registerreader.WithLogging(obsConfig.Logger))
	}
	return options
}

func buildCancelReaderContractOptions(obsConfig ObservabilityConfig) []cancelreadercontract.Option {
	var options []cancelreadercontract.Option
	if obsConfig.MetricsCollector != nil {
		options = append(options, cancelreadercontract.WithMetrics(obsConfig.MetricsCollector))
	}
	if obsConfig.TracingCollector != nil {
		options = append(options, cancelreadercontract.WithTracing(obsConfig.TracingCollector))
	}
	if obsConfig.ContextualLogger != nil {
		options = append(options, cancelreadercontract.WithContextualLogging(obsConfig.ContextualLogger))
	}
	if obsConfig.Logger != nil {
		options = append(options, cancelreadercontract.WithLogging(obsConfig.Logger))
	}
	return options
}

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

// buildBooksInCirculationOptions creates observability options for BooksInCirculation query handler.
func buildBooksInCirculationOptions(obsConfig ObservabilityConfig) []booksincirculation.Option {
	var options []booksincirculation.Option
	if obsConfig.MetricsCollector != nil {
		options = append(options, booksincirculation.WithMetrics(obsConfig.MetricsCollector))
	}
	if obsConfig.TracingCollector != nil {
		options = append(options, booksincirculation.WithTracing(obsConfig.TracingCollector))
	}
	if obsConfig.ContextualLogger != nil {
		options = append(options, booksincirculation.WithContextualLogging(obsConfig.ContextualLogger))
	}
	if obsConfig.Logger != nil {
		options = append(options, booksincirculation.WithLogging(obsConfig.Logger))
	}
	return options
}

// buildBooksLentOutOptions creates observability options for BooksLentOut query handler.
func buildBooksLentOutOptions(obsConfig ObservabilityConfig) []bookslentout.Option {
	var options []bookslentout.Option
	if obsConfig.MetricsCollector != nil {
		options = append(options, bookslentout.WithMetrics(obsConfig.MetricsCollector))
	}
	if obsConfig.TracingCollector != nil {
		options = append(options, bookslentout.WithTracing(obsConfig.TracingCollector))
	}
	if obsConfig.ContextualLogger != nil {
		options = append(options, bookslentout.WithContextualLogging(obsConfig.ContextualLogger))
	}
	if obsConfig.Logger != nil {
		options = append(options, bookslentout.WithLogging(obsConfig.Logger))
	}
	return options
}

// buildBooksLentByReaderOptions creates observability options for BooksLentByReader query handler.
func buildBooksLentByReaderOptions(obsConfig ObservabilityConfig) []bookslentbyreader.Option {
	var options []bookslentbyreader.Option
	if obsConfig.MetricsCollector != nil {
		options = append(options, bookslentbyreader.WithMetrics(obsConfig.MetricsCollector))
	}
	if obsConfig.TracingCollector != nil {
		options = append(options, bookslentbyreader.WithTracing(obsConfig.TracingCollector))
	}
	if obsConfig.ContextualLogger != nil {
		options = append(options, bookslentbyreader.WithContextualLogging(obsConfig.ContextualLogger))
	}
	if obsConfig.Logger != nil {
		options = append(options, bookslentbyreader.WithLogging(obsConfig.Logger))
	}
	return options
}

// buildRegisteredReadersOptions creates observability options for RegisteredReaders query handler.
func buildRegisteredReadersOptions(obsConfig ObservabilityConfig) []registeredreaders.Option {
	var options []registeredreaders.Option
	if obsConfig.MetricsCollector != nil {
		options = append(options, registeredreaders.WithMetrics(obsConfig.MetricsCollector))
	}
	if obsConfig.TracingCollector != nil {
		options = append(options, registeredreaders.WithTracing(obsConfig.TracingCollector))
	}
	if obsConfig.ContextualLogger != nil {
		options = append(options, registeredreaders.WithContextualLogging(obsConfig.ContextualLogger))
	}
	if obsConfig.Logger != nil {
		options = append(options, registeredreaders.WithLogging(obsConfig.Logger))
	}
	return options
}
