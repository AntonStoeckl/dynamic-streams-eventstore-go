package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/google/uuid"

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

// HandlerBundle contains all command and query handlers for the simulation.
type HandlerBundle struct {
	// Command handlers.
	addBookCopyHandler    addbookcopy.CommandHandler
	removeBookCopyHandler removebookcopy.CommandHandler
	registerReaderHandler registerreader.CommandHandler
	cancelReaderHandler   cancelreadercontract.CommandHandler
	lendBookCopyHandler   lendbookcopytoreader.CommandHandler
	returnBookCopyHandler returnbookcopyfromreader.CommandHandler

	// Query handlers for state refresh.
	booksInCirculationHandler booksincirculation.QueryHandler
	booksLentByReaderHandler  bookslentbyreader.QueryHandler
	booksLentOutHandler       bookslentout.QueryHandler
	registeredReadersHandler  registeredreaders.QueryHandler

	// Performance monitoring.
	loadController *LoadController

	// State access for actor decisions.
	simulationState *SimulationState
}

// NewHandlerBundle creates all command handlers with optional observability.
func NewHandlerBundle(eventStore *postgresengine.EventStore, cfg Config) (*HandlerBundle, error) {
	// Get observability config once if enabled.
	var obsConfig ObservabilityConfig
	if cfg.ObservabilityEnabled {
		obsConfig = cfg.NewObservabilityConfig()
		log.Printf("🔍 Observability enabled - metrics: %t, tracing: %t, logging: %t",
			obsConfig.MetricsCollector != nil,
			obsConfig.TracingCollector != nil,
			obsConfig.ContextualLogger != nil)
	}

	// Create command handlers with observability options if enabled.
	addBookCopyHandler, err := addbookcopy.NewCommandHandler(eventStore,
		buildAddBookCopyOptions(obsConfig)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create AddBookCopy handler: %w", err)
	}

	removeBookCopyHandler, err := removebookcopy.NewCommandHandler(eventStore,
		buildRemoveBookCopyOptions(obsConfig)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create RemoveBookCopy handler: %w", err)
	}

	registerReaderHandler, err := registerreader.NewCommandHandler(eventStore,
		buildRegisterReaderOptions(obsConfig)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create RegisterReader handler: %w", err)
	}

	cancelReaderHandler, err := cancelreadercontract.NewCommandHandler(eventStore,
		buildCancelReaderOptions(obsConfig)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create CancelReaderContract handler: %w", err)
	}

	lendBookCopyHandler, err := lendbookcopytoreader.NewCommandHandler(eventStore,
		buildLendBookCopyOptions(obsConfig)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create LendBookCopy handler: %w", err)
	}

	returnBookCopyHandler, err := returnbookcopyfromreader.NewCommandHandler(eventStore,
		buildReturnBookCopyOptions(obsConfig)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create ReturnBookCopy handler: %w", err)
	}

	// Create query handlers with observability options if enabled.
	booksInCirculationHandler, err := booksincirculation.NewQueryHandler(eventStore,
		buildBooksInCirculationOptions(obsConfig)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create BooksInCirculation handler: %w", err)
	}

	booksLentByReaderHandler, err := bookslentbyreader.NewQueryHandler(eventStore,
		buildBooksLentByReaderOptions(obsConfig)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create BooksLentByReader handler: %w", err)
	}

	booksLentOutHandler, err := bookslentout.NewQueryHandler(eventStore,
		buildBooksLentOutOptions(obsConfig)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create BooksLentOut handler: %w", err)
	}

	registeredReadersHandler, err := registeredreaders.NewQueryHandler(eventStore,
		buildRegisteredReadersOptions(obsConfig)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create RegisteredReaders handler: %w", err)
	}

	return &HandlerBundle{
		addBookCopyHandler:        addBookCopyHandler,
		removeBookCopyHandler:     removeBookCopyHandler,
		registerReaderHandler:     registerReaderHandler,
		cancelReaderHandler:       cancelReaderHandler,
		lendBookCopyHandler:       lendBookCopyHandler,
		returnBookCopyHandler:     returnBookCopyHandler,
		booksInCirculationHandler: booksInCirculationHandler,
		booksLentByReaderHandler:  booksLentByReaderHandler,
		booksLentOutHandler:       booksLentOutHandler,
		registeredReadersHandler:  registeredReadersHandler,
	}, nil
}

// SetLoadController sets the load controller for performance monitoring.
func (hb *HandlerBundle) SetLoadController(lc *LoadController) {
	hb.loadController = lc
}

// SetSimulationState sets the simulation state for actor decisions.
func (hb *HandlerBundle) SetSimulationState(state *SimulationState) {
	hb.simulationState = state
}

// GetSimulationState returns read-only access to simulation state for actor decisions.
func (hb *HandlerBundle) GetSimulationState() *SimulationState {
	return hb.simulationState
}

// recordMetrics records operation metrics for load controller if available.
func (hb *HandlerBundle) recordMetrics(ctx context.Context, start time.Time, err error) {
	if hb.loadController == nil {
		return
	}

	duration := time.Since(start)
	hb.loadController.RecordLatency(duration)

	// Log timeout/cancellation specifically for debugging
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		log.Printf("⏱️ TIMEOUT: Operation exceeded deadline after %v", duration)
	case errors.Is(err, context.Canceled):
		log.Printf("🚫 CANCELLED: Operation cancelled after %v", duration)
	case duration > 5*time.Second:
		log.Printf("🐌 SLOW: Operation took %v (no timeout)", duration)
	}

	// Check for timeout error (both context and error parameters).
	timedOut := ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
	hb.loadController.RecordTimeout(timedOut)
}

// ExecuteAddBook adds a book to circulation via command handler.
func (hb *HandlerBundle) ExecuteAddBook(ctx context.Context, bookID uuid.UUID) error {
	start := time.Now()
	timeoutCtx, cancel := context.WithTimeout(ctx, CommandTimeoutSeconds*time.Second)
	defer cancel()

	command := addbookcopy.BuildCommand(
		bookID,
		generateISBN(),
		generateBookTitle(),
		generateAuthor(),
		"1st Edition",
		"Simulation Press",
		2024,
		time.Now(),
	)
	err := hb.addBookCopyHandler.Handle(timeoutCtx, command)
	hb.recordMetrics(timeoutCtx, start, err)
	return err
}

// ExecuteRemoveBook removes a book from circulation via command handler.
func (hb *HandlerBundle) ExecuteRemoveBook(ctx context.Context, bookID uuid.UUID) error {
	start := time.Now()
	timeoutCtx, cancel := context.WithTimeout(ctx, CommandTimeoutSeconds*time.Second)
	defer cancel()

	command := removebookcopy.BuildCommand(bookID, time.Now())
	err := hb.removeBookCopyHandler.Handle(timeoutCtx, command)
	hb.recordMetrics(timeoutCtx, start, err)
	return err
}

// ExecuteRegisterReader registers a new reader via command handler.
func (hb *HandlerBundle) ExecuteRegisterReader(ctx context.Context, readerID uuid.UUID) error {
	start := time.Now()
	timeoutCtx, cancel := context.WithTimeout(ctx, CommandTimeoutSeconds*time.Second)
	defer cancel()

	command := registerreader.BuildCommand(
		readerID,
		generateReaderName(),
		time.Now(),
	)
	err := hb.registerReaderHandler.Handle(timeoutCtx, command)
	hb.recordMetrics(timeoutCtx, start, err)
	return err
}

// ExecuteCancelReader cancels a reader contract via command handler.
func (hb *HandlerBundle) ExecuteCancelReader(ctx context.Context, readerID uuid.UUID) error {
	start := time.Now()
	timeoutCtx, cancel := context.WithTimeout(ctx, CommandTimeoutSeconds*time.Second)
	defer cancel()

	command := cancelreadercontract.BuildCommand(readerID, time.Now())
	err := hb.cancelReaderHandler.Handle(timeoutCtx, command)
	hb.recordMetrics(timeoutCtx, start, err)
	return err
}

// ExecuteLendBook lends a book to a reader via command handler.
func (hb *HandlerBundle) ExecuteLendBook(ctx context.Context, bookID, readerID uuid.UUID) error {
	start := time.Now()
	timeoutCtx, cancel := context.WithTimeout(ctx, CommandTimeoutSeconds*time.Second)
	defer cancel()

	command := lendbookcopytoreader.BuildCommand(bookID, readerID, time.Now())
	err := hb.lendBookCopyHandler.Handle(timeoutCtx, command)
	hb.recordMetrics(timeoutCtx, start, err)
	return err
}

// ExecuteReturnBook returns a book from a reader via command handler.
func (hb *HandlerBundle) ExecuteReturnBook(ctx context.Context, bookID, readerID uuid.UUID) error {
	start := time.Now()
	timeoutCtx, cancel := context.WithTimeout(ctx, CommandTimeoutSeconds*time.Second)
	defer cancel()

	command := returnbookcopyfromreader.BuildCommand(bookID, readerID, time.Now())
	err := hb.returnBookCopyHandler.Handle(timeoutCtx, command)
	hb.recordMetrics(timeoutCtx, start, err)
	return err
}

// =================================================================
// STATE REFRESH QUERIES - Used to sync in-memory state with EventStore
// =================================================================

// QueryBooksInCirculation returns all books currently in circulation.
func (hb *HandlerBundle) QueryBooksInCirculation(ctx context.Context) (booksincirculation.BooksInCirculation, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, QueryTimeoutSeconds*time.Second)
	defer cancel()
	return hb.booksInCirculationHandler.Handle(timeoutCtx)
}

// QueryBooksLentByReader returns all books currently lent to a specific reader.
func (hb *HandlerBundle) QueryBooksLentByReader(ctx context.Context, readerID uuid.UUID) (bookslentbyreader.BooksCurrentlyLent, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, QueryTimeoutSeconds*time.Second)
	defer cancel()
	query := bookslentbyreader.Query{ReaderID: readerID}
	return hb.booksLentByReaderHandler.Handle(timeoutCtx, query)
}

// QueryBooksLentOut returns all books currently lent out.
func (hb *HandlerBundle) QueryBooksLentOut(ctx context.Context) (bookslentout.BooksLentOut, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, QueryTimeoutSeconds*time.Second)
	defer cancel()
	return hb.booksLentOutHandler.Handle(timeoutCtx)
}

// QueryRegisteredReaders returns all currently registered readers.
func (hb *HandlerBundle) QueryRegisteredReaders(ctx context.Context) (registeredreaders.RegisteredReaders, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, QueryTimeoutSeconds*time.Second)
	defer cancel()
	return hb.registeredReadersHandler.Handle(timeoutCtx)
}

// =================================================================
// STATE REFRESH QUERIES - For periodic state synchronization
// =================================================================

// QueryBooksInCirculationForState returns all books for state refresh.
func (hb *HandlerBundle) QueryBooksInCirculationForState(ctx context.Context) (booksincirculation.BooksInCirculation, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, QueryTimeoutSeconds*time.Second)
	defer cancel()
	return hb.booksInCirculationHandler.Handle(timeoutCtx)
}

// QueryRegisteredReadersForState returns all readers for state refresh.
func (hb *HandlerBundle) QueryRegisteredReadersForState(ctx context.Context) (registeredreaders.RegisteredReaders, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, QueryTimeoutSeconds*time.Second)
	defer cancel()
	return hb.registeredReadersHandler.Handle(timeoutCtx)
}

// =================================================================
// SIMULATION DATA GENERATORS
// =================================================================

var bookTitles = []string{
	"The Art of Go Programming", "Database Systems Fundamentals", "Event-Driven Architecture",
	"Microservices Patterns", "Distributed Systems Design", "Modern Software Architecture",
	"Clean Code Principles", "System Design Interview", "Algorithms and Data Structures",
}

var authors = []string{
	"Kent Beck", "Martin Fowler", "Eric Evans", "Sam Newman", "Pat Helland",
	"Werner Vogels", "Adrian Cockcroft", "Gregor Hohpe", "Michael Feathers",
}

var readerNames = []string{
	"Anna Schmidt", "Max Mueller", "Lisa Weber", "Tom Fischer", "Sarah Wagner",
	"Michael Bauer", "Julia Richter", "David Klein", "Maria Hoffmann", "Stefan Neumann",
}

func generateISBN() string {
	return fmt.Sprintf("978-%d-%d-%d-%d",
		1000+rand.Intn(9000), 100+rand.Intn(900), 100+rand.Intn(900), rand.Intn(10)) //nolint:gosec // Weak random OK for simulation
}

func generateBookTitle() string {
	return bookTitles[rand.Intn(len(bookTitles))] //nolint:gosec // Weak random OK for simulation
}

func generateAuthor() string {
	return authors[rand.Intn(len(authors))] //nolint:gosec // Weak random OK for simulation
}

func generateReaderName() string {
	return readerNames[rand.Intn(len(readerNames))] //nolint:gosec // Weak random OK for simulation
}

// =================================================================
// OBSERVABILITY OPTIONS BUILDERS
// =================================================================

func buildAddBookCopyOptions(obsConfig ObservabilityConfig) []addbookcopy.Option {
	var opts []addbookcopy.Option
	if obsConfig.MetricsCollector != nil {
		opts = append(opts, addbookcopy.WithMetrics(obsConfig.MetricsCollector))
	}
	if obsConfig.TracingCollector != nil {
		opts = append(opts, addbookcopy.WithTracing(obsConfig.TracingCollector))
	}
	if obsConfig.ContextualLogger != nil {
		opts = append(opts, addbookcopy.WithContextualLogging(obsConfig.ContextualLogger))
	}
	if obsConfig.Logger != nil {
		opts = append(opts, addbookcopy.WithLogging(obsConfig.Logger))
	}
	return opts
}

func buildRemoveBookCopyOptions(obsConfig ObservabilityConfig) []removebookcopy.Option {
	var opts []removebookcopy.Option
	if obsConfig.MetricsCollector != nil {
		opts = append(opts, removebookcopy.WithMetrics(obsConfig.MetricsCollector))
	}
	if obsConfig.TracingCollector != nil {
		opts = append(opts, removebookcopy.WithTracing(obsConfig.TracingCollector))
	}
	if obsConfig.ContextualLogger != nil {
		opts = append(opts, removebookcopy.WithContextualLogging(obsConfig.ContextualLogger))
	}
	if obsConfig.Logger != nil {
		opts = append(opts, removebookcopy.WithLogging(obsConfig.Logger))
	}
	return opts
}

func buildRegisterReaderOptions(obsConfig ObservabilityConfig) []registerreader.Option {
	var opts []registerreader.Option
	if obsConfig.MetricsCollector != nil {
		opts = append(opts, registerreader.WithMetrics(obsConfig.MetricsCollector))
	}
	if obsConfig.TracingCollector != nil {
		opts = append(opts, registerreader.WithTracing(obsConfig.TracingCollector))
	}
	if obsConfig.ContextualLogger != nil {
		opts = append(opts, registerreader.WithContextualLogging(obsConfig.ContextualLogger))
	}
	if obsConfig.Logger != nil {
		opts = append(opts, registerreader.WithLogging(obsConfig.Logger))
	}
	return opts
}

func buildCancelReaderOptions(obsConfig ObservabilityConfig) []cancelreadercontract.Option {
	var opts []cancelreadercontract.Option
	if obsConfig.MetricsCollector != nil {
		opts = append(opts, cancelreadercontract.WithMetrics(obsConfig.MetricsCollector))
	}
	if obsConfig.TracingCollector != nil {
		opts = append(opts, cancelreadercontract.WithTracing(obsConfig.TracingCollector))
	}
	if obsConfig.ContextualLogger != nil {
		opts = append(opts, cancelreadercontract.WithContextualLogging(obsConfig.ContextualLogger))
	}
	if obsConfig.Logger != nil {
		opts = append(opts, cancelreadercontract.WithLogging(obsConfig.Logger))
	}
	return opts
}

func buildLendBookCopyOptions(obsConfig ObservabilityConfig) []lendbookcopytoreader.Option {
	var opts []lendbookcopytoreader.Option
	if obsConfig.MetricsCollector != nil {
		opts = append(opts, lendbookcopytoreader.WithMetrics(obsConfig.MetricsCollector))
	}
	if obsConfig.TracingCollector != nil {
		opts = append(opts, lendbookcopytoreader.WithTracing(obsConfig.TracingCollector))
	}
	if obsConfig.ContextualLogger != nil {
		opts = append(opts, lendbookcopytoreader.WithContextualLogging(obsConfig.ContextualLogger))
	}
	if obsConfig.Logger != nil {
		opts = append(opts, lendbookcopytoreader.WithLogging(obsConfig.Logger))
	}
	return opts
}

func buildReturnBookCopyOptions(obsConfig ObservabilityConfig) []returnbookcopyfromreader.Option {
	var opts []returnbookcopyfromreader.Option
	if obsConfig.MetricsCollector != nil {
		opts = append(opts, returnbookcopyfromreader.WithMetrics(obsConfig.MetricsCollector))
	}
	if obsConfig.TracingCollector != nil {
		opts = append(opts, returnbookcopyfromreader.WithTracing(obsConfig.TracingCollector))
	}
	if obsConfig.ContextualLogger != nil {
		opts = append(opts, returnbookcopyfromreader.WithContextualLogging(obsConfig.ContextualLogger))
	}
	if obsConfig.Logger != nil {
		opts = append(opts, returnbookcopyfromreader.WithLogging(obsConfig.Logger))
	}
	return opts
}

func buildBooksInCirculationOptions(obsConfig ObservabilityConfig) []booksincirculation.Option {
	var opts []booksincirculation.Option
	if obsConfig.MetricsCollector != nil {
		opts = append(opts, booksincirculation.WithMetrics(obsConfig.MetricsCollector))
	}
	if obsConfig.TracingCollector != nil {
		opts = append(opts, booksincirculation.WithTracing(obsConfig.TracingCollector))
	}
	if obsConfig.ContextualLogger != nil {
		opts = append(opts, booksincirculation.WithContextualLogging(obsConfig.ContextualLogger))
	}
	if obsConfig.Logger != nil {
		opts = append(opts, booksincirculation.WithLogging(obsConfig.Logger))
	}
	return opts
}

func buildBooksLentByReaderOptions(obsConfig ObservabilityConfig) []bookslentbyreader.Option {
	var opts []bookslentbyreader.Option
	if obsConfig.MetricsCollector != nil {
		opts = append(opts, bookslentbyreader.WithMetrics(obsConfig.MetricsCollector))
	}
	if obsConfig.TracingCollector != nil {
		opts = append(opts, bookslentbyreader.WithTracing(obsConfig.TracingCollector))
	}
	if obsConfig.ContextualLogger != nil {
		opts = append(opts, bookslentbyreader.WithContextualLogging(obsConfig.ContextualLogger))
	}
	if obsConfig.Logger != nil {
		opts = append(opts, bookslentbyreader.WithLogging(obsConfig.Logger))
	}
	return opts
}

func buildBooksLentOutOptions(obsConfig ObservabilityConfig) []bookslentout.Option {
	var opts []bookslentout.Option
	if obsConfig.MetricsCollector != nil {
		opts = append(opts, bookslentout.WithMetrics(obsConfig.MetricsCollector))
	}
	if obsConfig.TracingCollector != nil {
		opts = append(opts, bookslentout.WithTracing(obsConfig.TracingCollector))
	}
	if obsConfig.ContextualLogger != nil {
		opts = append(opts, bookslentout.WithContextualLogging(obsConfig.ContextualLogger))
	}
	if obsConfig.Logger != nil {
		opts = append(opts, bookslentout.WithLogging(obsConfig.Logger))
	}
	return opts
}

func buildRegisteredReadersOptions(obsConfig ObservabilityConfig) []registeredreaders.Option {
	var opts []registeredreaders.Option
	if obsConfig.MetricsCollector != nil {
		opts = append(opts, registeredreaders.WithMetrics(obsConfig.MetricsCollector))
	}
	if obsConfig.TracingCollector != nil {
		opts = append(opts, registeredreaders.WithTracing(obsConfig.TracingCollector))
	}
	if obsConfig.ContextualLogger != nil {
		opts = append(opts, registeredreaders.WithContextualLogging(obsConfig.ContextualLogger))
	}
	if obsConfig.Logger != nil {
		opts = append(opts, registeredreaders.WithLogging(obsConfig.Logger))
	}
	return opts
}
