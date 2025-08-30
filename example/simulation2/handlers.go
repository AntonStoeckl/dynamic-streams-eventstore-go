package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
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
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell/observable"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell/snapshot"
)

// HandlerBundle contains all command and query handlers for the simulation.
type HandlerBundle struct {
	// Command handlers.
	addBookCopyHandler    *observable.CommandWrapper[addbookcopy.Command]
	removeBookCopyHandler *observable.CommandWrapper[removebookcopy.Command]
	registerReaderHandler *observable.CommandWrapper[registerreader.Command]
	cancelReaderHandler   *observable.CommandWrapper[cancelreadercontract.Command]
	lendBookCopyHandler   *observable.CommandWrapper[lendbookcopytoreader.Command]
	returnBookCopyHandler *observable.CommandWrapper[returnbookcopyfromreader.Command]

	// Query handlers for state refresh.
	booksInCirculationHandler *snapshot.GenericSnapshotWrapper[booksincirculation.Query, booksincirculation.BooksInCirculation]
	booksLentByReaderHandler  *snapshot.GenericSnapshotWrapper[bookslentbyreader.Query, bookslentbyreader.BooksCurrentlyLent]
	booksLentOutHandler       *snapshot.GenericSnapshotWrapper[bookslentout.Query, bookslentout.BooksLentOut]
	registeredReadersHandler  *snapshot.GenericSnapshotWrapper[registeredreaders.Query, registeredreaders.RegisteredReaders]

	// State access for actor decisions.
	simulationState *SimulationState
}

// NewHandlerBundle creates all command handlers with optional observability.
//
//nolint:funlen
func NewHandlerBundle(eventStore *postgresengine.EventStore, cfg Config) (*HandlerBundle, error) {
	// Get observability config once if enabled.
	var obsConfig ObservabilityConfig
	if cfg.ObservabilityEnabled {
		obsConfig = cfg.newObservabilityConfig()
		log.Printf("🔍 Observability enabled - metrics: %t, tracing: %t, logging: %t",
			obsConfig.MetricsCollector != nil,
			obsConfig.TracingCollector != nil,
			obsConfig.ContextualLogger != nil)
	}

	// Build observability options from config
	opts := buildObservabilityOptions(obsConfig)

	// Create command handlers.
	// Command handlers are wrapped with observability if enabled.
	coreAddBookCopyHandler := addbookcopy.NewCommandHandler(eventStore)
	addBookCopyHandler, err := observable.NewCommandWrapper(coreAddBookCopyHandler, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create AddBookCopy handler: %w", err)
	}

	coreRemoveBookCopyHandler := removebookcopy.NewCommandHandler(eventStore)
	removeBookCopyHandler, err := observable.NewCommandWrapper(coreRemoveBookCopyHandler, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to wrap RemoveBookCopy handler with observability: %w", err)
	}

	coreRegisterReaderHandler := registerreader.NewCommandHandler(eventStore)
	registerReaderHandler, err := observable.NewCommandWrapper(coreRegisterReaderHandler, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to wrap RegisterReader handler with observability: %w", err)
	}

	coreCancelReaderHandler := cancelreadercontract.NewCommandHandler(eventStore)
	cancelReaderHandler, err := observable.NewCommandWrapper(coreCancelReaderHandler, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to wrap CancelReaderContract handler with observability: %w", err)
	}

	coreLendBookCopyHandler := lendbookcopytoreader.NewCommandHandler(eventStore)

	lendBookCopyHandler, err := observable.NewCommandWrapper(coreLendBookCopyHandler, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to wrap LendBookCopy handler with observability: %w", err)
	}

	coreReturnBookCopyHandler := returnbookcopyfromreader.NewCommandHandler(eventStore)
	returnBookCopyHandler, err := observable.NewCommandWrapper(coreReturnBookCopyHandler, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to wrap ReturnBookCopyFromReader handler with observability: %w", err)
	}

	// Create query handlers.
	// Query handlers are wrapped with snapshotting if enabled.
	booksInCirculationBaseHandler, err := booksincirculation.NewQueryHandler(eventStore, buildBooksInCirculationOptions(obsConfig)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create BooksInCirculation handler: %w", err)
	}

	booksInCirculationHandler, err := snapshot.NewGenericSnapshotWrapper[
		booksincirculation.Query,
		booksincirculation.BooksInCirculation,
	](
		booksInCirculationBaseHandler,
		booksincirculation.Project,
		func(_ booksincirculation.Query) eventstore.Filter {
			return booksincirculation.BuildEventFilter()
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create BooksInCirculation snapshot wrapper: %w", err)
	}

	booksLentByReaderBaseHandler, err := bookslentbyreader.NewQueryHandler(eventStore, buildBooksLentByReaderOptions(obsConfig)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create BooksLentByReader handler: %w", err)
	}

	booksLentByReaderHandler, err := snapshot.NewGenericSnapshotWrapper[
		bookslentbyreader.Query,
		bookslentbyreader.BooksCurrentlyLent,
	](
		booksLentByReaderBaseHandler,
		bookslentbyreader.Project,
		func(q bookslentbyreader.Query) eventstore.Filter {
			return bookslentbyreader.BuildEventFilter(q.ReaderID)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create BooksLentByReader snapshot wrapper: %w", err)
	}

	booksLentOutBaseHandler, err := bookslentout.NewQueryHandler(eventStore, buildBooksLentOutOptions(obsConfig)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create BooksLentOut handler: %w", err)
	}

	booksLentOutHandler, err := snapshot.NewGenericSnapshotWrapper[
		bookslentout.Query,
		bookslentout.BooksLentOut,
	](
		booksLentOutBaseHandler,
		bookslentout.Project,
		func(_ bookslentout.Query) eventstore.Filter {
			return bookslentout.BuildEventFilter()
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create BooksLentOut snapshot wrapper: %w", err)
	}

	registeredReadersBaseHandler, err := registeredreaders.NewQueryHandler(eventStore, buildRegisteredReadersOptions(obsConfig)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create RegisteredReaders handler: %w", err)
	}

	registeredReadersHandler, err := snapshot.NewGenericSnapshotWrapper[
		registeredreaders.Query,
		registeredreaders.RegisteredReaders,
	](
		registeredReadersBaseHandler,
		registeredreaders.Project,
		func(_ registeredreaders.Query) eventstore.Filter {
			return registeredreaders.BuildEventFilter()
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create RegisteredReaders snapshot wrapper: %w", err)
	}

	bundle := &HandlerBundle{
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
	}

	return bundle, nil
}

// SetSimulationState sets the simulation state for actor decisions.
func (hb *HandlerBundle) SetSimulationState(state *SimulationState) {
	hb.simulationState = state
}

// GetSimulationState returns read-only access to simulation state for actor decisions.
func (hb *HandlerBundle) GetSimulationState() *SimulationState {
	return hb.simulationState
}

// buildObservabilityOptions creates observable wrapper options from the config.
func buildObservabilityOptions(obsConfig ObservabilityConfig) []observable.Option {
	var opts []observable.Option
	if obsConfig.MetricsCollector != nil {
		opts = append(opts, observable.WithMetrics(obsConfig.MetricsCollector))
	}
	if obsConfig.TracingCollector != nil {
		opts = append(opts, observable.WithTracing(obsConfig.TracingCollector))
	}
	if obsConfig.ContextualLogger != nil {
		opts = append(opts, observable.WithContextualLogging(obsConfig.ContextualLogger))
	}
	if obsConfig.Logger != nil {
		opts = append(opts, observable.WithLogging(obsConfig.Logger))
	}
	return opts
}

// recordMetrics logs operation timing and errors (simplified for simulation).
func (hb *HandlerBundle) recordMetrics(ctx context.Context, start time.Time, err error) {
	duration := time.Since(start)

	// Log timeout/cancellation specifically for debugging (keep this synchronous)
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		// Get the timeout type from context metadata
		timeoutType, hasType := GetTimeoutType(ctx)

		if hasType {
			switch timeoutType {
			case CommandTimeoutType:
				log.Printf("%s %s", SystemError("⏱️"),
					SystemError(fmt.Sprintf("COMMAND TIMEOUT: Operation exceeded %ds deadline after %v",
						int(CommandTimeoutSeconds), duration)))
			case BatchTimeoutType:
				log.Printf("%s %s", SystemError("⏱️"),
					SystemError(fmt.Sprintf("BATCH TIMEOUT: Operation exceeded %ds deadline after %v",
						int(BatchTimeoutSeconds), duration)))
			}
		} else {
			// Fallback for contexts without metadata
			log.Printf("%s %s", SystemError("⏱️"),
				SystemError(fmt.Sprintf("TIMEOUT: Operation exceeded deadline after %v", duration)))
		}

	case errors.Is(err, context.Canceled):
		log.Printf("%s %s", SystemError("🚫"),
			SystemError(fmt.Sprintf("CANCELED: Operation canceled after %v", duration)))

	case duration > 5*time.Second:
		log.Printf("🐌 SLOW: Operation took %v (no timeout)", duration)
	}

	// Note: Simulation gets metrics directly from batch processing
	// No need to send metrics to LoadController
}

// ExecuteAddBook adds a book to circulation via command handler.
func (hb *HandlerBundle) ExecuteAddBook(ctx context.Context, bookID uuid.UUID) error {
	start := time.Now()
	timeoutCtx, cancel := WithCommandTimeout(ctx, time.Duration(CommandTimeoutSeconds*float64(time.Second)))
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
	_, err := hb.addBookCopyHandler.Handle(timeoutCtx, command)
	hb.recordMetrics(timeoutCtx, start, err)

	// Update simulation state after a successful book addition
	if err == nil && hb.simulationState != nil {
		hb.simulationState.AddBook(bookID)
	}

	return err
}

// ExecuteRemoveBook removes a book from circulation via command handler.
func (hb *HandlerBundle) ExecuteRemoveBook(ctx context.Context, bookID uuid.UUID) error {
	start := time.Now()
	timeoutCtx, cancel := WithCommandTimeout(ctx, time.Duration(CommandTimeoutSeconds*float64(time.Second)))
	defer cancel()

	command := removebookcopy.BuildCommand(bookID, time.Now())
	_, err := hb.removeBookCopyHandler.Handle(timeoutCtx, command)
	hb.recordMetrics(timeoutCtx, start, err)

	// Update simulation state after successful book removal
	if err == nil && hb.simulationState != nil {
		hb.simulationState.RemoveBook(bookID)
	}

	return err
}

// ExecuteRegisterReader registers a new reader via command handler.
func (hb *HandlerBundle) ExecuteRegisterReader(ctx context.Context, readerID uuid.UUID) error {
	start := time.Now()
	timeoutCtx, cancel := WithCommandTimeout(ctx, time.Duration(CommandTimeoutSeconds*float64(time.Second)))
	defer cancel()

	command := registerreader.BuildCommand(
		readerID,
		generateReaderName(),
		time.Now(),
	)
	_, err := hb.registerReaderHandler.Handle(timeoutCtx, command)
	hb.recordMetrics(timeoutCtx, start, err)

	// Update simulation state after successful reader registration
	if err == nil && hb.simulationState != nil {
		hb.simulationState.RegisterReader(readerID)
	}

	return err
}

// ExecuteCancelReader cancels a reader contract via command handler.
func (hb *HandlerBundle) ExecuteCancelReader(ctx context.Context, readerID uuid.UUID) error {
	start := time.Now()
	timeoutCtx, cancel := WithCommandTimeout(ctx, time.Duration(CommandTimeoutSeconds*float64(time.Second)))
	defer cancel()

	command := cancelreadercontract.BuildCommand(readerID, time.Now())
	_, err := hb.cancelReaderHandler.Handle(timeoutCtx, command)
	hb.recordMetrics(timeoutCtx, start, err)

	// Update simulation state after successful reader cancellation
	if err == nil && hb.simulationState != nil {
		hb.simulationState.CancelReader(readerID)
	}

	return err
}

// ExecuteLendBook lends a book to a reader via command handler.
func (hb *HandlerBundle) ExecuteLendBook(ctx context.Context, bookID, readerID uuid.UUID) error {
	start := time.Now()
	timeoutCtx, cancel := WithCommandTimeout(ctx, time.Duration(CommandTimeoutSeconds*float64(time.Second)))
	defer cancel()

	command := lendbookcopytoreader.BuildCommand(bookID, readerID, time.Now())
	_, err := hb.lendBookCopyHandler.Handle(timeoutCtx, command)
	hb.recordMetrics(timeoutCtx, start, err)

	// Update simulation state after successful lending
	if err == nil && hb.simulationState != nil {
		hb.simulationState.LendBook(bookID, readerID)
	}

	return err
}

// ExecuteReturnBook returns a book from a reader via command handler.
func (hb *HandlerBundle) ExecuteReturnBook(ctx context.Context, bookID, readerID uuid.UUID) error {
	start := time.Now()
	timeoutCtx, cancel := WithCommandTimeout(ctx, time.Duration(CommandTimeoutSeconds*float64(time.Second)))
	defer cancel()

	command := returnbookcopyfromreader.BuildCommand(bookID, readerID, time.Now())
	_, err := hb.returnBookCopyHandler.Handle(timeoutCtx, command)
	hb.recordMetrics(timeoutCtx, start, err)

	// Update simulation state after successful return
	if err == nil && hb.simulationState != nil {
		hb.simulationState.ReturnBook(bookID, readerID)
	}

	return err
}

// =================================================================
// STATE REFRESH QUERIES - Used to sync in-memory state with EventStore
// =================================================================

// QueryBooksInCirculation returns all books currently in circulation.
// Without metrics reporting, as this query is slow.
func (hb *HandlerBundle) QueryBooksInCirculation(ctx context.Context) (booksincirculation.BooksInCirculation, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(BooksInCirculationQueryTimeoutSeconds*float64(time.Second)))
	defer cancel()
	result, err := hb.booksInCirculationHandler.Handle(timeoutCtx, booksincirculation.Query{})

	return result, err
}

// QueryBooksLentByReader returns all books currently lent to a specific reader.
// With metrics reporting, as this query is fast.
func (hb *HandlerBundle) QueryBooksLentByReader(ctx context.Context, readerID uuid.UUID) (bookslentbyreader.BooksCurrentlyLent, error) {
	start := time.Now()
	timeoutCtx, cancel := WithCommandTimeout(ctx, time.Duration(BooksLentByReaderQueryTimeoutSeconds*float64(time.Second)))
	defer cancel()
	query := bookslentbyreader.Query{ReaderID: readerID}
	result, err := hb.booksLentByReaderHandler.Handle(timeoutCtx, query)
	hb.recordMetrics(timeoutCtx, start, err)

	return result, err
}

// QueryBooksLentOut returns all books currently lent out.
// Without metrics reporting, as this query is slow.
func (hb *HandlerBundle) QueryBooksLentOut(ctx context.Context) (bookslentout.BooksLentOut, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(BooksLentOutQueryTimeoutSeconds*float64(time.Second)))
	defer cancel()
	result, err := hb.booksLentOutHandler.Handle(timeoutCtx, bookslentout.Query{})

	return result, err
}

// QueryRegisteredReaders returns all currently registered readers.
// Without metrics reporting, as this query is slow.
func (hb *HandlerBundle) QueryRegisteredReaders(ctx context.Context) (registeredreaders.RegisteredReaders, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(RegisteredReadersQueryTimeoutSeconds*float64(time.Second)))
	defer cancel()
	result, err := hb.registeredReadersHandler.Handle(timeoutCtx, registeredreaders.Query{})

	return result, err
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
