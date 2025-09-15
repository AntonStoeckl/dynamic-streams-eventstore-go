// Package main provides a data consistency analysis and cleanup tool for the EventStore simulation.
// It identifies and optionally fixes orphaned lendings and ghost books in the library management system.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/oteladapters"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/returnbookcopyfromreader"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/query/booksincirculation"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/query/bookslentout"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/query/canceledreaders"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/query/finishedlendings"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/query/registeredreaders"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/query/removedbooks"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell/config"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell/observable"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell/snapshot"
)

// Config holds the command-line configuration for the cleanup tool.
type Config struct {
	ObservabilityEnabled   bool
	PerformStateCleanup    bool
	DeleteCanceledReaders  bool
	DeleteRemovedBooks     bool
	DeleteFinishedLendings bool
	CanceledReadersLimit   int
	RemovedBooksLimit      int
	FinishedLendingsLimit  int
	VerboseOutput          bool
}

// ObservabilityConfig holds the observability adapters for query handlers.
type ObservabilityConfig struct {
	Logger           eventstore.Logger
	ContextualLogger eventstore.ContextualLogger
	MetricsCollector eventstore.MetricsCollector
	TracingCollector eventstore.TracingCollector
}

// ErrorStats tracks different types of command failures.
type ErrorStats struct {
	BookNeverInCirculation     int
	BookRemovedFromCirculation int
	ReaderNeverRegistered      int
	BookNotLentToReader        int
	OtherErrors                int
	Successes                  int
	VerboseDetails             []VerboseError
	VerboseEnabled             bool
	mu                         sync.Mutex
}

// VerboseError holds detailed error information for verbose output.
type VerboseError struct {
	BookID   string
	ReaderID string
	Error    string
	Type     string
}

// RecordError categorizes and counts different error types.
func (e *ErrorStats) RecordError(errorMsg, bookID, readerID string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	var errorType string
	switch {
	case strings.Contains(errorMsg, "book was never added to circulation"):
		e.BookNeverInCirculation++
		errorType = "BookNeverInCirculation"
	case strings.Contains(errorMsg, "book was removed from circulation"):
		e.BookRemovedFromCirculation++
		errorType = "BookRemovedFromCirculation"
	case strings.Contains(errorMsg, "reader was never registered"):
		e.ReaderNeverRegistered++
		errorType = "ReaderNeverRegistered"
	case strings.Contains(errorMsg, "book is not lent to this reader"):
		e.BookNotLentToReader++
		errorType = "BookNotLentToReader"
	default:
		e.OtherErrors++
		errorType = "OtherError"
	}

	if e.VerboseEnabled {
		e.VerboseDetails = append(e.VerboseDetails, VerboseError{
			BookID:   bookID,
			ReaderID: readerID,
			Error:    errorMsg,
			Type:     errorType,
		})
	}
}

// RecordSuccess increments the success counter.
func (e *ErrorStats) RecordSuccess(bookID, readerID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Successes++

	if e.VerboseEnabled {
		e.VerboseDetails = append(e.VerboseDetails, VerboseError{
			BookID:   bookID,
			ReaderID: readerID,
			Error:    "Success",
			Type:     "Success",
		})
	}
}

// PrintSummary displays the error statistics.
func (e *ErrorStats) PrintSummary() {
	e.mu.Lock()
	defer e.mu.Unlock()

	total := e.BookNeverInCirculation + e.BookRemovedFromCirculation + e.ReaderNeverRegistered + e.BookNotLentToReader + e.OtherErrors + e.Successes
	if total == 0 {
		return
	}

	fmt.Printf("\n%s %s\n", StatusIcon("stats"), Header("ERROR TYPE BREAKDOWN"))
	fmt.Printf("%s\n", Separator("=", 24))
	if e.Successes > 0 {
		fmt.Printf("%s %s %s %s\n", Success("✅"), BrightGreen("Successful returns:"), Bold(fmt.Sprintf("%d", e.Successes)), Gray(fmt.Sprintf("(%.1f%%)", float64(e.Successes)*100.0/float64(total))))
	}
	if e.BookNeverInCirculation > 0 {
		fmt.Printf("%s %s %s %s\n", Error("❌"), BrightRed("Book never in circulation:"), Bold(fmt.Sprintf("%d", e.BookNeverInCirculation)), Gray(fmt.Sprintf("(%.1f%%)", float64(e.BookNeverInCirculation)*100.0/float64(total))))
	}
	if e.BookRemovedFromCirculation > 0 {
		fmt.Printf("%s %s %s %s\n", StatusIcon("ghost"), BrightMagenta("Book removed from circulation:"), Bold(fmt.Sprintf("%d", e.BookRemovedFromCirculation)), Gray(fmt.Sprintf("(%.1f%%)", float64(e.BookRemovedFromCirculation)*100.0/float64(total))))
	}
	if e.ReaderNeverRegistered > 0 {
		fmt.Printf("🚫 %s %s %s\n", BrightRed("Reader never registered:"), Bold(fmt.Sprintf("%d", e.ReaderNeverRegistered)), Gray(fmt.Sprintf("(%.1f%%)", float64(e.ReaderNeverRegistered)*100.0/float64(total))))
	}
	if e.BookNotLentToReader > 0 {
		fmt.Printf("%s %s %s %s\n", StatusIcon("books"), BrightYellow("Book not lent to reader:"), Bold(fmt.Sprintf("%d", e.BookNotLentToReader)), Gray(fmt.Sprintf("(%.1f%%)", float64(e.BookNotLentToReader)*100.0/float64(total))))
	}
	if e.OtherErrors > 0 {
		fmt.Printf("❓ %s %s %s\n", BrightRed("Other errors:"), Bold(fmt.Sprintf("%d", e.OtherErrors)), Gray(fmt.Sprintf("(%.1f%%)", float64(e.OtherErrors)*100.0/float64(total))))
	}

	// Print verbose details if enabled
	if e.VerboseEnabled && len(e.VerboseDetails) > 0 {
		log.Printf("\n🔍 DETAILED BREAKDOWN:")
		log.Printf("===================")

		// Group by error type for better organization
		errorGroups := make(map[string][]VerboseError)
		for _, detail := range e.VerboseDetails {
			errorGroups[detail.Type] = append(errorGroups[detail.Type], detail)
		}

		for errorType, errors := range errorGroups {
			if errorType == "Success" {
				continue // Skip successes in verbose output
			}

			log.Printf("\n%s (%d items):", errorType, len(errors))
			for _, err := range errors {
				if err.ReaderID != "" {
					log.Printf("  Book: %s, Reader: %s - %s", err.BookID, err.ReaderID, err.Error)
				} else {
					log.Printf("  Book: %s - %s", err.BookID, err.Error)
				}
			}
		}
	}
}

// identifySafeToDeleteReaders filters out readers that have active lendings to prevent creating orphaned lendings.
func identifySafeToDeleteReaders(canceledReaders []canceledreaders.ReaderInfo, activeLendings []bookslentout.LendingInfo) []canceledreaders.ReaderInfo {
	activeLendingsByReader := make(map[string]bool)
	for _, lending := range activeLendings {
		activeLendingsByReader[lending.ReaderID] = true
	}

	var safeToDelete []canceledreaders.ReaderInfo
	for _, reader := range canceledReaders {
		if !activeLendingsByReader[reader.ReaderID] {
			safeToDelete = append(safeToDelete, reader)
		}
	}

	return safeToDelete
}

// identifySafeToDeleteBooks filters out books that have active lendings to prevent creating ghost lendings.
func identifySafeToDeleteBooks(removedBooks []removedbooks.BookInfo, activeLendings []bookslentout.LendingInfo) []removedbooks.BookInfo {
	activeLendingsByBook := make(map[string]bool)
	for _, lending := range activeLendings {
		activeLendingsByBook[lending.BookID] = true
	}

	var safeToDelete []removedbooks.BookInfo
	for _, book := range removedBooks {
		if !activeLendingsByBook[book.BookID] {
			safeToDelete = append(safeToDelete, book)
		}
	}

	return safeToDelete
}

// calculateEventEstimations provides estimated event counts based on lending activity.
func calculateEventEstimations(
	registeredReadersCount, canceledReadersCount int,
	booksInCirculationCount, removedBooksCount int,
	openLendingsCount, finishedLendingsCount int,
) (avgEventsPerReader, avgEventsPerBook int) {
	totalReaders := registeredReadersCount + canceledReadersCount
	totalBooks := booksInCirculationCount + removedBooksCount
	totalLendingActivity := openLendingsCount + finishedLendingsCount

	if totalReaders > 0 {
		// Each lending = 2 events (lent and returned), plus 2 base events (registration and cancellation)
		avgLendingsPerReader := totalLendingActivity / totalReaders
		avgEventsPerReader = (avgLendingsPerReader * 2) + 2
	}

	if totalBooks > 0 {
		// Each lending = 2 events (lent and returned), plus 2 base events (add and remove from circulation)
		avgLendingsPerBook := totalLendingActivity / totalBooks
		avgEventsPerBook = (avgLendingsPerBook * 2) + 2
	}

	return avgEventsPerReader, avgEventsPerBook
}

// deleteReaderEvents deletes all events for a specific reader using JSONB query.
func deleteReaderEvents(ctx context.Context, pool *pgxpool.Pool, readerID string) (int, error) {
	query := `DELETE FROM events WHERE payload @> $1`

	// Create a JSONB object with ReaderID
	payloadFilter := fmt.Sprintf(`{"ReaderID": "%s"}`, readerID)

	result, err := pool.Exec(ctx, query, payloadFilter)
	if err != nil {
		return 0, fmt.Errorf("failed to delete events for reader %s: %w", readerID, err)
	}

	deletedCount := int(result.RowsAffected())
	return deletedCount, nil
}

// deleteBookEvents deletes all events for a specific book using JSONB query.
func deleteBookEvents(ctx context.Context, pool *pgxpool.Pool, bookID string) (int, error) {
	query := `DELETE FROM events WHERE payload @> $1`

	// Create JSONB object with BookID
	payloadFilter := fmt.Sprintf(`{"BookID": "%s"}`, bookID)

	result, err := pool.Exec(ctx, query, payloadFilter)
	if err != nil {
		return 0, fmt.Errorf("failed to delete events for book %s: %w", bookID, err)
	}

	deletedCount := int(result.RowsAffected())
	return deletedCount, nil
}

// deleteFinishedLendingEvents deletes events for a slice of finished lendings.
// Each lending should have exactly 2 events: BookCopyLentToReader and BookCopyReturnedByReader.
func deleteFinishedLendingEvents(ctx context.Context, pool *pgxpool.Pool, lendings []finishedlendings.LendingInfo) (int, error) {
	if len(lendings) == 0 {
		return 0, nil
	}

	totalDeleted := 0

	// Process lendings in batches for better performance
	const batchSize = 100
	for i := 0; i < len(lendings); i += batchSize {
		end := i + batchSize
		if end > len(lendings) {
			end = len(lendings)
		}

		batch := lendings[i:end]
		batchDeleted, err := deleteFinishedLendingBatch(ctx, pool, batch)
		if err != nil {
			return totalDeleted, fmt.Errorf("failed to delete batch %d-%d: %w", i, end-1, err)
		}

		totalDeleted += batchDeleted

		// Log progress for large batches
		if len(lendings) > 500 && (i+batchSize)%500 == 0 {
			log.Printf("Progress: Deleted events for %d/%d lendings", i+batchSize, len(lendings))
		}
	}

	// Removed debug output
	return totalDeleted, nil
}

// deleteFinishedLendingBatch deletes events for a batch of finished lendings.
func deleteFinishedLendingBatch(ctx context.Context, pool *pgxpool.Pool, batch []finishedlendings.LendingInfo) (int, error) {
	if len(batch) == 0 {
		return 0, nil
	}

	totalDeleted := 0

	// Process each lending individually to avoid complex dynamic SQL
	for _, lending := range batch {
		// Delete both events for this book-reader pair in a single query
		query := `DELETE FROM events 
			WHERE event_type IN ('BookCopyLentToReader', 'BookCopyReturnedByReader')
			AND payload @> $1`

		// Create a JSONB object with both BookID and ReaderID
		payloadFilter := fmt.Sprintf(`{"BookID": "%s", "ReaderID": "%s"}`, lending.BookID, lending.ReaderID)

		result, err := pool.Exec(ctx, query, payloadFilter)
		if err != nil {
			return totalDeleted, fmt.Errorf("failed to delete events for lending %s-%s: %w", lending.BookID, lending.ReaderID, err)
		}

		deleted := int(result.RowsAffected())
		totalDeleted += deleted

		// Only warn if we deleted something other than 0 or 2 events
		// 0 events = already deleted (expected after multiple runs)
		// 2 events = normal case
		if deleted != 0 && deleted != 2 {
			log.Printf("WARNING: Expected 0 or 2 events for lending %s-%s, deleted %d", lending.BookID, lending.ReaderID, deleted)
		}
	}

	return totalDeleted, nil
}

//nolint:gocognit,gocyclo,funlen // Main function orchestrates entire cleanup workflow - complexity justified for a single-purpose tool
func main() {
	fmt.Printf("%s %s\n", StatusIcon("debug"), Header("Data Consistency Analysis Tool"))
	fmt.Printf("%s\n", Separator("=", 33))

	cfg := parseFlags()

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Hour)
	defer cancel()

	// Initialize EventStore (same as simulation)
	eventStore, pgxPool, err := initializePGXEventStore(ctx, cfg)
	if err != nil {
		log.Panicf("Failed to initialize EventStore: %v", err)
	}

	// Create query handlers with observability if enabled
	var obsConfig ObservabilityConfig
	if cfg.ObservabilityEnabled {
		obsConfig = cfg.NewObservabilityConfig()
		log.Printf("🔍 Observability enabled - metrics: %t, tracing: %t, logging: %t",
			obsConfig.MetricsCollector != nil,
			obsConfig.TracingCollector != nil,
			obsConfig.ContextualLogger != nil)
	}

	// RegisteredReaders: Core → Snapshot → Observable
	registeredReadersCoreHandler := registeredreaders.NewQueryHandler(eventStore)

	registeredReadersSnapshotHandler, err := snapshot.NewQueryWrapper[
		registeredreaders.Query,
		registeredreaders.RegisteredReaders,
	](
		registeredReadersCoreHandler,
		eventStore,
		registeredreaders.Project,
		func(_ registeredreaders.Query) eventstore.Filter {
			return registeredreaders.BuildEventFilter()
		},
	)
	if err != nil {
		log.Panicf("Failed to create RegisteredReaders snapshot wrapper: %v", err)
	}

	registeredReadersHandler, err := observable.NewQueryWrapper(
		registeredReadersSnapshotHandler,
		buildRegisteredReadersQueryOptions(obsConfig)...,
	)
	if err != nil {
		log.Panicf("Failed to create RegisteredReaders observable wrapper: %v", err)
	}

	// BooksLentOut: Core → Snapshot → Observable
	booksLentOutCoreHandler := bookslentout.NewQueryHandler(eventStore)

	booksLentOutSnapshotHandler, err := snapshot.NewQueryWrapper[
		bookslentout.Query,
		bookslentout.BooksLentOut,
	](
		booksLentOutCoreHandler,
		eventStore,
		bookslentout.Project,
		func(_ bookslentout.Query) eventstore.Filter {
			return bookslentout.BuildEventFilter()
		},
	)
	if err != nil {
		log.Panicf("Failed to create BooksLentOut snapshot wrapper: %v", err)
	}

	booksLentOutHandler, err := observable.NewQueryWrapper(
		booksLentOutSnapshotHandler,
		buildBooksLentOutQueryOptions(obsConfig)...,
	)
	if err != nil {
		log.Panicf("Failed to create BooksLentOut observable wrapper: %v", err)
	}

	// Create BooksInCirculation handler using new composition pattern
	// Core Handler -> Snapshot Wrapper -> Observable Wrapper

	baseBooksInCirculationHandler := booksincirculation.NewQueryHandler(eventStore)

	booksInCirculationSnapshotHandler, err := snapshot.NewQueryWrapper[
		booksincirculation.Query,
		booksincirculation.BooksInCirculation,
	](
		baseBooksInCirculationHandler,
		eventStore,
		booksincirculation.Project,
		func(_ booksincirculation.Query) eventstore.Filter {
			return booksincirculation.BuildEventFilter()
		},
	)
	if err != nil {
		log.Panicf("Failed to create BooksInCirculation snapshot wrapper: %v", err)
	}

	booksInCirculationHandler, err := observable.NewQueryWrapper[
		booksincirculation.Query,
		booksincirculation.BooksInCirculation,
	](
		booksInCirculationSnapshotHandler,
		buildBooksInCirculationQueryOptions(obsConfig)...,
	)
	if err != nil {
		log.Panicf("Failed to create BooksInCirculation observable wrapper: %v", err)
	}

	canceledReadersCoreHandler := canceledreaders.NewQueryHandler(eventStore)

	canceledReadersSnapshotHandler, err := snapshot.NewQueryWrapper[
		canceledreaders.Query,
		canceledreaders.CanceledReaders,
	](
		canceledReadersCoreHandler,
		eventStore,
		canceledreaders.Project,
		func(_ canceledreaders.Query) eventstore.Filter {
			return canceledreaders.BuildEventFilter()
		},
	)
	if err != nil {
		log.Panicf("Failed to create CanceledReaders snapshot wrapper: %v", err)
	}

	canceledReadersHandler, err := observable.NewQueryWrapper(
		canceledReadersSnapshotHandler,
		buildQueryOptions[canceledreaders.Query, canceledreaders.CanceledReaders](obsConfig)...,
	)
	if err != nil {
		log.Panicf("Failed to create CanceledReaders observable wrapper: %v", err)
	}

	// RemovedBooks: Core → Snapshot → Observable
	removedBooksCoreHandler := removedbooks.NewQueryHandler(eventStore)

	removedBooksSnapshotHandler, err := snapshot.NewQueryWrapper[
		removedbooks.Query,
		removedbooks.RemovedBooks,
	](
		removedBooksCoreHandler,
		eventStore,
		removedbooks.Project,
		func(_ removedbooks.Query) eventstore.Filter {
			return removedbooks.BuildEventFilter()
		},
	)
	if err != nil {
		log.Panicf("Failed to create RemovedBooks snapshot wrapper: %v", err)
	}

	removedBooksHandler, err := observable.NewQueryWrapper(
		removedBooksSnapshotHandler,
		buildQueryOptions[removedbooks.Query, removedbooks.RemovedBooks](obsConfig)...,
	)
	if err != nil {
		log.Panicf("Failed to create RemovedBooks observable wrapper: %v", err)
	}

	// FinishedLendings: Core → Snapshot → Observable
	finishedLendingsCoreHandler := finishedlendings.NewQueryHandler(eventStore)

	finishedLendingsSnapshotHandler, err := snapshot.NewQueryWrapper[
		finishedlendings.Query,
		finishedlendings.FinishedLendings,
	](
		finishedLendingsCoreHandler,
		eventStore,
		finishedlendings.Project,
		func(_ finishedlendings.Query) eventstore.Filter {
			return finishedlendings.BuildEventFilter()
		},
	)
	if err != nil {
		log.Panicf("Failed to create FinishedLendings snapshot wrapper: %v", err)
	}

	finishedLendingsHandler, err := observable.NewQueryWrapper(
		finishedLendingsSnapshotHandler,
		buildQueryOptions[finishedlendings.Query, finishedlendings.FinishedLendings](obsConfig)...,
	)
	if err != nil {
		log.Panicf("Failed to create FinishedLendings observable wrapper: %v", err)
	}

	// Query all data
	fmt.Printf("%s %s\n", StatusIcon("stats"), Info("Querying database state..."))

	// 1. Get all registered readers
	registeredReadersResult, err := registeredReadersHandler.Handle(ctx, registeredreaders.Query{})
	if err != nil {
		log.Panicf("Failed to query registered readers: %v", err)
	}

	// 2. Get all books lent out
	booksLentOutResult, err := booksLentOutHandler.Handle(ctx, bookslentout.Query{})
	if err != nil {
		log.Panicf("Failed to query books lent out: %v", err)
	}

	// 3. Get all books in circulation
	booksInCirculationResult, err := booksInCirculationHandler.Handle(ctx, booksincirculation.Query{})
	if err != nil {
		log.Panicf("Failed to query books in circulation: %v", err)
	}

	// 4. Get all canceled readers
	canceledReadersResult, err := canceledReadersHandler.Handle(ctx, canceledreaders.Query{})
	if err != nil {
		log.Panicf("Failed to query canceled readers: %v", err)
	}

	// 5. Get all removed books
	removedBooksResult, err := removedBooksHandler.Handle(ctx, removedbooks.Query{})
	if err != nil {
		log.Panicf("Failed to query removed books: %v", err)
	}

	// 6. Get finished lendings (use configured limit)
	//nolint:gosec // cfg.FinishedLendingsLimit is bounds-checked in parseFlags()
	finishedLendingsResult, err := finishedLendingsHandler.Handle(ctx, finishedlendings.BuildQuery(uint32(cfg.FinishedLendingsLimit)))
	if err != nil {
		log.Panicf("Failed to query finished lendings: %v", err)
	}

	// Enhanced Analysis with Safety Checks
	fmt.Printf("\n%s %s\n", StatusIcon("stats"), Header("ANALYSIS RESULTS"))
	fmt.Printf("%s\n", Separator("=", 18))

	// Active entities
	fmt.Printf("%s %s\n", Success("✅"), Header("Active Entities:"))
	fmt.Printf("%s %s %s\n", StatusIcon("readers"), BrightCyan("Registered readers:"), Bold(fmt.Sprintf("%d", len(registeredReadersResult.Readers))))
	fmt.Printf("%s %s %s\n", StatusIcon("books"), BrightCyan("Books in circulation:"), Bold(fmt.Sprintf("%d", len(booksInCirculationResult.Books))))
	fmt.Printf("🔗 %s %s\n", BrightCyan("Open lendings:"), Bold(fmt.Sprintf("%d", len(booksLentOutResult.Lendings))))

	// Calculate event estimations (use TotalCount for real total, not limited Count)
	avgEventsPerReader, avgEventsPerBook := calculateEventEstimations(
		len(registeredReadersResult.Readers), len(canceledReadersResult.Readers),
		len(booksInCirculationResult.Books), len(removedBooksResult.Books),
		len(booksLentOutResult.Lendings), finishedLendingsResult.TotalCount,
	)

	// Identify safe deletions
	safeToDeleteReaders := identifySafeToDeleteReaders(canceledReadersResult.Readers, booksLentOutResult.Lendings)
	unsafeReaderCount := len(canceledReadersResult.Readers) - len(safeToDeleteReaders)

	safeToDeleteBooks := identifySafeToDeleteBooks(removedBooksResult.Books, booksLentOutResult.Lendings)
	unsafeBookCount := len(removedBooksResult.Books) - len(safeToDeleteBooks)

	// Logically deleted entities
	fmt.Printf("\n%s %s\n", Error("❌"), Header("Logically Deleted Entities:"))

	// Canceled readers
	fmt.Printf("%s %s %s\n", StatusIcon("readers"), BrightMagenta("Canceled readers:"), Bold(fmt.Sprintf("%d total", len(canceledReadersResult.Readers))))

	if cfg.DeleteCanceledReaders {
		actualDeleteCount := cfg.CanceledReadersLimit
		if actualDeleteCount > len(safeToDeleteReaders) {
			actualDeleteCount = len(safeToDeleteReaders)
		}

		if actualDeleteCount > 0 {
			estimatedReaderEvents := actualDeleteCount * avgEventsPerReader
			fmt.Printf("   %s %s %s %s\n", Success("→"), BrightGreen("Planning to delete:"), Bold(fmt.Sprintf("%d oldest", actualDeleteCount)), Gray(fmt.Sprintf("(~%d events estimated)", estimatedReaderEvents)))
		}

		if actualDeleteCount < cfg.CanceledReadersLimit {
			fmt.Printf("   %s %s\n", Warning("→"), BrightYellow(fmt.Sprintf("Note: Only %d safe readers available (requested %d)", actualDeleteCount, cfg.CanceledReadersLimit)))
		}

		if unsafeReaderCount > 0 {
			fmt.Printf("   %s %s %s %s\n", Error("→"), BrightRed("UNSAFE to delete:"), Bold(fmt.Sprintf("%d", unsafeReaderCount)), Gray("(have active lendings - would create orphans!)"))
		}
	} else {
		if len(safeToDeleteReaders) > 0 {
			estimatedReaderEvents := len(safeToDeleteReaders) * avgEventsPerReader
			fmt.Printf("   %s %s %s %s\n", Success("→"), BrightGreen("Safe to delete:"), Bold(fmt.Sprintf("%d", len(safeToDeleteReaders))), Gray(fmt.Sprintf("(~%d events estimated)", estimatedReaderEvents)))
		}
		if unsafeReaderCount > 0 {
			fmt.Printf("   %s %s %s %s\n", Error("→"), BrightRed("UNSAFE to delete:"), Bold(fmt.Sprintf("%d", unsafeReaderCount)), Gray("(have active lendings - would create orphans!)"))
		}
		fmt.Printf("   %s %s\n", Gray("→"), Gray("Use --delete-canceled-readers with --canceled-readers-limit=N"))
	}

	// Removed books
	fmt.Printf("%s %s %s\n", StatusIcon("books"), BrightMagenta("Removed books:"), Bold(fmt.Sprintf("%d total", len(removedBooksResult.Books))))

	if cfg.DeleteRemovedBooks {
		actualDeleteCount := cfg.RemovedBooksLimit
		if actualDeleteCount > len(safeToDeleteBooks) {
			actualDeleteCount = len(safeToDeleteBooks)
		}

		if actualDeleteCount > 0 {
			estimatedBookEvents := actualDeleteCount * avgEventsPerBook
			fmt.Printf("   %s %s %s %s\n", Success("→"), BrightGreen("Planning to delete:"), Bold(fmt.Sprintf("%d oldest", actualDeleteCount)), Gray(fmt.Sprintf("(~%d events estimated)", estimatedBookEvents)))
		}

		if actualDeleteCount < cfg.RemovedBooksLimit {
			fmt.Printf("   %s %s\n", Warning("→"), BrightYellow(fmt.Sprintf("Note: Only %d safe books available (requested %d)", actualDeleteCount, cfg.RemovedBooksLimit)))
		}

		if unsafeBookCount > 0 {
			fmt.Printf("   %s %s %s %s\n", Error("→"), BrightRed("UNSAFE to delete:"), Bold(fmt.Sprintf("%d", unsafeBookCount)), Gray("(have active lendings - would create ghosts!)"))
		}
	} else {
		if len(safeToDeleteBooks) > 0 {
			estimatedBookEvents := len(safeToDeleteBooks) * avgEventsPerBook
			fmt.Printf("   %s %s %s %s\n", Success("→"), BrightGreen("Safe to delete:"), Bold(fmt.Sprintf("%d", len(safeToDeleteBooks))), Gray(fmt.Sprintf("(~%d events estimated)", estimatedBookEvents)))
		}
		if unsafeBookCount > 0 {
			fmt.Printf("   %s %s %s %s\n", Error("→"), BrightRed("UNSAFE to delete:"), Bold(fmt.Sprintf("%d", unsafeBookCount)), Gray("(have active lendings - would create ghosts!)"))
		}
		fmt.Printf("   %s %s\n", Gray("→"), Gray("Use --delete-removed-books with --removed-books-limit=N"))
	}

	// Finished lendings
	fmt.Printf("🔗 %s %s %s\n",
		BrightCyan("Finished lendings:"),
		Bold(fmt.Sprintf("%d total", finishedLendingsResult.TotalCount)),
		Gray(fmt.Sprintf("(showing %d oldest)", len(finishedLendingsResult.Lendings))))

	if cfg.DeleteFinishedLendings {
		actualDeleteCount := cfg.FinishedLendingsLimit
		if actualDeleteCount > finishedLendingsResult.TotalCount {
			actualDeleteCount = finishedLendingsResult.TotalCount
		}

		fmt.Printf("   %s %s %s %s\n",
			Success("→"),
			BrightGreen("Planning to delete:"),
			Bold(fmt.Sprintf("%d oldest", actualDeleteCount)),
			Gray(fmt.Sprintf("(exactly %d events)", actualDeleteCount*2)))

		if len(finishedLendingsResult.Lendings) > 0 && actualDeleteCount > 0 {
			oldestDate := finishedLendingsResult.Lendings[0].ReturnedAt.Format("2006-01-02")
			displayCount := min(actualDeleteCount, len(finishedLendingsResult.Lendings))
			if displayCount > 1 {
				newestDate := finishedLendingsResult.Lendings[displayCount-1].ReturnedAt.Format("2006-01-02")
				fmt.Printf("   %s %s %s %s %s\n", Gray("→"), Gray("Date range:"), Bold(oldestDate), Gray("to"), Bold(newestDate))
			} else {
				fmt.Printf("   %s %s %s\n", Gray("→"), Gray("Date:"), Bold(oldestDate))
			}
		}

		if actualDeleteCount < cfg.FinishedLendingsLimit {
			fmt.Printf("   %s %s\n", Warning("→"), BrightYellow(fmt.Sprintf("Note: Only %d available (requested %d)", actualDeleteCount, cfg.FinishedLendingsLimit)))
		}
	} else {
		totalFinishedLendingsEvents := finishedLendingsResult.TotalCount * 2
		fmt.Printf("   %s %s %s\n", Success("→"), BrightGreen("All safe to delete (completed cycles)"), Gray(fmt.Sprintf("(exactly %d events)", totalFinishedLendingsEvents)))
		fmt.Printf("   %s %s\n", Gray("→"), Gray("Use --delete-finished-lendings with --finished-lendings-limit=N"))
	}

	// Event estimations summary
	fmt.Printf("\n%s %s\n", StatusIcon("stats"), Header("Event Estimations:"))
	fmt.Printf("📊 %s %s\n", BrightCyan("Avg events per reader:"), Bold(fmt.Sprintf("~%d", avgEventsPerReader)))
	fmt.Printf("📊 %s %s\n", BrightCyan("Avg events per book:"), Bold(fmt.Sprintf("~%d", avgEventsPerBook)))

	fmt.Printf("\n%s %s\n", Warning("⚠️"), BrightYellow("Safety: Only showing entities safe to delete (no active lendings)"))

	// Execute deletion workflows if requested (before inconsistency analysis)
	var anyDeletionPerformed bool

	if cfg.DeleteCanceledReaders {
		anyDeletionPerformed = true

		// Apply limit to canceled readers (already sorted oldest first)
		readersToDelete := safeToDeleteReaders
		if cfg.CanceledReadersLimit < len(readersToDelete) {
			readersToDelete = readersToDelete[:cfg.CanceledReadersLimit]
		}

		if err := executeCanceledReadersWorkflow(ctx, pgxPool, readersToDelete); err != nil {
			log.Panicf("Failed to delete canceled readers: %v", err)
		}
		if err := clearCanceledReadersSnapshots(ctx, pgxPool); err != nil {
			log.Printf("WARNING: Failed to clear CanceledReaders snapshots after deletion: %v", err)
		}
	}

	if cfg.DeleteRemovedBooks {
		anyDeletionPerformed = true

		// Apply limit to removed books (already sorted oldest first)
		booksToDelete := safeToDeleteBooks
		if cfg.RemovedBooksLimit < len(booksToDelete) {
			booksToDelete = booksToDelete[:cfg.RemovedBooksLimit]
		}

		if err := executeRemovedBooksWorkflow(ctx, pgxPool, booksToDelete); err != nil {
			log.Panicf("Failed to delete removed books: %v", err)
		}
		if err := clearRemovedBooksSnapshots(ctx, pgxPool); err != nil {
			log.Printf("WARNING: Failed to clear RemovedBooks snapshots after deletion: %v", err)
		}
	}

	if cfg.DeleteFinishedLendings {
		anyDeletionPerformed = true
		lendings := finishedLendingsResult.Lendings
		limit := cfg.FinishedLendingsLimit
		if limit < len(lendings) {
			lendings = lendings[:limit]
		}
		if err := executeFinishedLendingsWorkflow(ctx, pgxPool, lendings, limit); err != nil {
			log.Panicf("Failed to delete finished lendings: %v", err)
		}
		if err := clearFinishedLendingsSnapshots(ctx, pgxPool); err != nil {
			log.Printf("WARNING: Failed to clear FinishedLendings snapshots after deletion: %v", err)
		}
	}

	if anyDeletionPerformed {
		fmt.Printf("\n🎉 %s\n", Success("Deletion operations completed! Run analysis again to see the results."))
		return
	}

	// Create lookup maps for analysis
	registeredReaderMap := make(map[string]bool)
	for _, reader := range registeredReadersResult.Readers {
		registeredReaderMap[reader.ReaderID] = true
	}

	circulatingBookMap := make(map[string]bool)
	for _, book := range booksInCirculationResult.Books {
		circulatingBookMap[book.BookID] = true
	}

	// Analyze orphaned lendings (reader canceled with open lending)
	orphanedLendings := 0
	var orphanedReaders []string
	readerLendingCount := make(map[string]int)

	for _, lending := range booksLentOutResult.Lendings {
		readerLendingCount[lending.ReaderID]++

		if !registeredReaderMap[lending.ReaderID] {
			orphanedLendings++
			// Track unique orphaned readers
			found := false
			for _, r := range orphanedReaders {
				if r == lending.ReaderID {
					found = true
					break
				}
			}
			if !found {
				orphanedReaders = append(orphanedReaders, lending.ReaderID)
			}
		}
	}

	// Analyze ghost lendings (book removed with open lending)
	ghostLendings := 0
	var ghostBooks []string

	for _, lending := range booksLentOutResult.Lendings {
		if !circulatingBookMap[lending.BookID] {
			ghostLendings++
			// Track unique ghost books
			found := false
			for _, b := range ghostBooks {
				if b == lending.BookID {
					found = true
					break
				}
			}
			if !found {
				ghostBooks = append(ghostBooks, lending.BookID)
			}
		}
	}

	// Report findings
	fmt.Printf("\n🚨 %s\n", Header("INCONSISTENCY ANALYSIS"))
	fmt.Printf("%s\n", Separator("=", 25))
	fmt.Printf("%s %s %s\n", Error("❌"), BrightRed("Orphaned lendings (reader cancelled with open lending):"), Bold(fmt.Sprintf("%d", orphanedLendings)))
	fmt.Printf("   %s %s %s %s\n", Gray("→"), Gray("Affecting"), Bold(fmt.Sprintf("%d", len(orphanedReaders))), Gray("unique cancelled readers"))
	fmt.Printf("%s %s %s\n", StatusIcon("ghost"), BrightMagenta("Ghost lendings (book removed with open lending):"), Bold(fmt.Sprintf("%d", ghostLendings)))
	fmt.Printf("   %s %s %s %s\n", Gray("→"), Gray("Affecting"), Bold(fmt.Sprintf("%d", len(ghostBooks))), Gray("unique removed books"))

	// Show some examples
	if len(orphanedReaders) > 0 {
		log.Printf("\n📋 Sample orphaned readers:")
		for i, readerID := range orphanedReaders {
			if i >= 3 { // Show only the first 3
				log.Printf("   ... and %d more", len(orphanedReaders)-3)
				break
			}
			count := readerLendingCount[readerID]
			log.Printf("   - %s (%d books)", readerID, count)
		}
	}

	if len(ghostBooks) > 0 {
		log.Printf("\n📋 Sample ghost books:")
		for i, bookID := range ghostBooks {
			if i >= 3 { // Show only the first 3
				log.Printf("   ... and %d more", len(ghostBooks)-3)
				break
			}
			log.Printf("   - %s", bookID)
		}
	}

	// Summary
	totalInconsistencies := orphanedLendings + ghostLendings
	fmt.Printf("\n%s %s\n", StatusIcon("stats"), Header("SUMMARY"))
	fmt.Printf("%s\n", Separator("=", 10))
	fmt.Printf("%s %s %s\n", Success("✅"), BrightGreen("Consistent lendings:"), Bold(fmt.Sprintf("%d", len(booksLentOutResult.Lendings)-totalInconsistencies)))
	fmt.Printf("%s %s %s\n", Error("❌"), BrightRed("Inconsistent lendings:"), Bold(fmt.Sprintf("%d", totalInconsistencies)))
	if len(booksLentOutResult.Lendings) > 0 {
		consistency := float64(len(booksLentOutResult.Lendings)-totalInconsistencies) * 100.0 / float64(len(booksLentOutResult.Lendings))
		color := BrightGreen
		if consistency < 95 {
			color = BrightYellow
		}
		if consistency < 90 {
			color = BrightRed
		}
		fmt.Printf("📈 %s %s\n", BrightCyan("Data consistency:"), color(fmt.Sprintf("%.1f%%", consistency)))
	}

	if totalInconsistencies == 0 {
		fmt.Printf("🎉 %s\n", Success("Database is fully consistent!"))
		return
	}
	fmt.Printf("%s %s %s %s\n", Warning("⚠️"), BrightYellow("Database requires cleanup to resolve"), Bold(fmt.Sprintf("%d", totalInconsistencies)), BrightYellow("inconsistencies"))

	// Perform cleanup if requested
	if !cfg.PerformStateCleanup {
		log.Printf("💡 To perform state cleanup, run with --fix-state flag")
		log.Printf("💡 Example: ./cleanup --fix-state --observability-enabled")
		log.Printf("💡 For detailed output: ./cleanup --fix-state --verbose")
		return
	}

	if orphanedLendings == 0 {
		log.Printf("ℹ️  No orphaned lendings to clean up")
		return
	}

	log.Printf("\n🧹 CLEANUP OPERATIONS")
	log.Printf("====================")
	log.Printf("⚠️  About to clean up %d orphaned lendings for %d cancelled readers", orphanedLendings, len(orphanedReaders))
	log.Printf("💡 Creating ReturnBookCopyFromReader commands for each orphaned lending...")

	// Create a command handler for cleanup
	returnBookHandler := returnbookcopyfromreader.NewCommandHandler(eventStore)

	// Collect all orphaned lendings for parallel processing
	orphanedLendingsList := make([]struct{ BookID, ReaderID string }, 0, orphanedLendings)
	for _, lending := range booksLentOutResult.Lendings {
		if !registeredReaderMap[lending.ReaderID] {
			orphanedLendingsList = append(orphanedLendingsList, struct{ BookID, ReaderID string }{
				BookID:   lending.BookID,
				ReaderID: lending.ReaderID,
			})
		}
	}

	// Process cleanup in parallel with 25 goroutines
	cleanupCtx, cleanupCancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cleanupCancel()

	processed, failed := performParallelCleanup(cleanupCtx, &returnBookHandler, orphanedLendingsList, cfg.VerboseOutput)

	log.Printf("\n✅ CLEANUP COMPLETE")
	log.Printf("==================")
	log.Printf("🔄 Processed: %d orphaned lendings", processed)
	log.Printf("❌ Failed: %d operations", failed)
	log.Printf("✅ Success rate: %.1f%%", float64(processed-failed)*100.0/float64(processed))

	if failed > 0 {
		log.Printf("⚠️  Some orphaned lending operations failed. Re-run the tool to check remaining inconsistencies.")
	} else {
		log.Printf("🎉 All orphaned lendings cleaned up successfully!")
	}

	// Now cleanup ghost lendings (books removed while lent out)
	if ghostLendings == 0 {
		log.Printf("ℹ️  No ghost lendings to clean up")
		return
	}

	log.Printf("\n👻 GHOST BOOK CLEANUP")
	log.Printf("====================")
	log.Printf("⚠️  About to force-return %d ghost books (books removed while lent out)", ghostLendings)
	log.Printf("💡 Creating ReturnBookCopyFromReader commands for each ghost lending...")

	// Collect all ghost lendings for parallel processing
	ghostLendingsList := make([]struct{ BookID, ReaderID string }, 0, ghostLendings)
	for _, lending := range booksLentOutResult.Lendings {
		if !circulatingBookMap[lending.BookID] {
			ghostLendingsList = append(ghostLendingsList, struct{ BookID, ReaderID string }{
				BookID:   lending.BookID,
				ReaderID: lending.ReaderID,
			})
		}
	}

	// Process ghost cleanup in parallel with 25 goroutines
	ghostCtx, ghostCancel := context.WithTimeout(ctx, 5*time.Minute)
	defer ghostCancel()

	ghostProcessed, ghostFailed := performParallelCleanup(ghostCtx, &returnBookHandler, ghostLendingsList, cfg.VerboseOutput)

	log.Printf("\n✅ GHOST CLEANUP COMPLETE")
	log.Printf("========================")
	log.Printf("🔄 Processed: %d ghost lendings", ghostProcessed)
	log.Printf("❌ Failed: %d operations", ghostFailed)
	log.Printf("✅ Success rate: %.1f%%", float64(ghostProcessed-ghostFailed)*100.0/float64(ghostProcessed))

	if ghostFailed > 0 {
		log.Printf("⚠️  Some ghost lending operations failed. Re-run the tool to check remaining inconsistencies.")
	} else {
		log.Printf("🎉 All ghost lendings cleaned up successfully!")
	}

	// Final summary
	totalProcessed := processed + ghostProcessed
	totalFailed := failed + ghostFailed
	log.Printf("\n🎯 TOTAL CLEANUP SUMMARY")
	log.Printf("=======================")
	log.Printf("🔄 Total operations: %d", totalProcessed)
	log.Printf("✅ Successful: %d", totalProcessed-totalFailed)
	log.Printf("❌ Failed: %d", totalFailed)
	log.Printf("📈 Overall success rate: %.1f%%", float64(totalProcessed-totalFailed)*100.0/float64(totalProcessed))

	if totalFailed == 0 {
		log.Printf("🏆 Perfect cleanup! All inconsistencies resolved!")
		log.Printf("💡 Run analysis again to verify database consistency.")
	}
}

// executeCanceledReadersWorkflow implements the delete-canceled-readers workflow.
func executeCanceledReadersWorkflow(ctx context.Context, pool *pgxpool.Pool, safeReaders []canceledreaders.ReaderInfo) error {
	if len(safeReaders) == 0 {
		fmt.Printf("\n%s %s\n", StatusIcon("info"), BrightCyan("No safe canceled readers to delete"))
		return nil
	}

	fmt.Printf("\n%s %s\n", StatusIcon("delete"), Header("DELETE CANCELED READERS"))
	fmt.Printf("%s\n", Separator("=", 24))
	fmt.Printf("📋 %s %s %s\n", BrightCyan("Ready to delete:"), Bold(fmt.Sprintf("%d", len(safeReaders))), Gray("canceled readers"))

	// Confirmation prompt
	fmt.Printf("\n%s %s ", Warning("⚠️"), BrightYellow("This will permanently delete all events for these readers. Continue? (y/N):"))
	var response string
	_, _ = fmt.Scanln(&response)
	if response != "y" && response != "Y" {
		fmt.Printf("❌ Operation cancelled\n")
		return nil
	}

	fmt.Printf("🚀 Starting deletion process...\n")
	totalDeleted := 0
	failed := 0

	// Process readers with progress updates
	for i, reader := range safeReaders {
		deleted, err := deleteReaderEvents(ctx, pool, reader.ReaderID)
		if err != nil {
			log.Printf("❌ Failed to delete reader %s: %v", reader.ReaderID, err)
			failed++
			continue
		}

		totalDeleted += deleted

		// Progress updates every 100 operations
		if (i+1)%100 == 0 || i == len(safeReaders)-1 {
			fmt.Printf("Progress: %d/%d readers processed\n", i+1, len(safeReaders))
		}
	}

	// Summary report
	fmt.Printf("\n✅ %s\n", Header("DELETION SUMMARY"))
	fmt.Printf("📊 Readers processed: %d\n", len(safeReaders))
	fmt.Printf("📊 Events deleted: %d\n", totalDeleted)
	fmt.Printf("❌ Failed operations: %d\n", failed)
	if len(safeReaders) > 0 {
		fmt.Printf("📈 Success rate: %.1f%%\n", float64(len(safeReaders)-failed)*100.0/float64(len(safeReaders)))
	}

	return nil
}

// executeRemovedBooksWorkflow implements the delete-removed-books workflow.
func executeRemovedBooksWorkflow(ctx context.Context, pool *pgxpool.Pool, safeBooks []removedbooks.BookInfo) error {
	if len(safeBooks) == 0 {
		fmt.Printf("\n%s %s\n", StatusIcon("info"), BrightCyan("No safe removed books to delete"))
		return nil
	}

	fmt.Printf("\n%s %s\n", StatusIcon("delete"), Header("DELETE REMOVED BOOKS"))
	fmt.Printf("%s\n", Separator("=", 20))
	fmt.Printf("📋 %s %s %s\n", BrightCyan("Ready to delete:"), Bold(fmt.Sprintf("%d", len(safeBooks))), Gray("removed books"))

	// Confirmation prompt
	fmt.Printf("\n%s %s ", Warning("⚠️"), BrightYellow("This will permanently delete all events for these books. Continue? (y/N):"))
	var response string
	_, _ = fmt.Scanln(&response)
	if response != "y" && response != "Y" {
		fmt.Printf("❌ Operation cancelled\n")
		return nil
	}

	fmt.Printf("🚀 Starting deletion process...\n")
	totalDeleted := 0
	failed := 0

	// Process books with progress updates
	for i, book := range safeBooks {
		deleted, err := deleteBookEvents(ctx, pool, book.BookID)
		if err != nil {
			log.Printf("❌ Failed to delete book %s: %v", book.BookID, err)
			failed++
			continue
		}

		totalDeleted += deleted

		// Progress updates every 100 operations
		if (i+1)%100 == 0 || i == len(safeBooks)-1 {
			fmt.Printf("Progress: %d/%d books processed\n", i+1, len(safeBooks))
		}
	}

	// Summary report
	fmt.Printf("\n✅ %s\n", Header("DELETION SUMMARY"))
	fmt.Printf("📊 Books processed: %d\n", len(safeBooks))
	fmt.Printf("📊 Events deleted: %d\n", totalDeleted)
	fmt.Printf("❌ Failed operations: %d\n", failed)
	if len(safeBooks) > 0 {
		fmt.Printf("📈 Success rate: %.1f%%\n", float64(len(safeBooks)-failed)*100.0/float64(len(safeBooks)))
	}

	return nil
}

// executeFinishedLendingsWorkflow implements the delete-finished-lendings workflow.
func executeFinishedLendingsWorkflow(ctx context.Context, pool *pgxpool.Pool, lendings []finishedlendings.LendingInfo, _ int) error {
	if len(lendings) == 0 {
		fmt.Printf("\n%s %s\n", StatusIcon("info"), BrightCyan("No finished lendings to delete"))
		return nil
	}

	fmt.Printf("\n%s %s\n", StatusIcon("delete"), Header("DELETE FINISHED LENDINGS"))
	fmt.Printf("%s\n", Separator("=", 24))
	fmt.Printf("📋 %s %s %s %s\n", BrightCyan("Ready to delete:"), Bold(fmt.Sprintf("%d", len(lendings))), Gray("finished lendings"), Gray(fmt.Sprintf("(exactly %d events)", len(lendings)*2)))

	if len(lendings) > 0 {
		oldestDate := lendings[0].ReturnedAt.Format("2006-01-02")
		if len(lendings) > 1 {
			newestDate := lendings[len(lendings)-1].ReturnedAt.Format("2006-01-02")
			fmt.Printf("📅 Date range: %s to %s\n", oldestDate, newestDate)
		} else {
			fmt.Printf("📅 Date: %s\n", oldestDate)
		}
	}

	// Confirmation prompt
	fmt.Printf("\n%s %s ", Warning("⚠️"), BrightYellow("This will permanently delete lending events. Continue? (y/N):"))
	var response string
	_, _ = fmt.Scanln(&response)
	if response != "y" && response != "Y" {
		fmt.Printf("❌ Operation cancelled\n")
		return nil
	}

	fmt.Printf("🚀 Starting deletion process...\n")

	deleted, err := deleteFinishedLendingEvents(ctx, pool, lendings)
	if err != nil {
		return fmt.Errorf("deletion failed: %w", err)
	}

	// Verify exactly 2 events per lending
	expected := len(lendings) * 2
	if deleted != expected {
		log.Printf("⚠️  WARNING: Expected %d events, deleted %d", expected, deleted)
	}

	// Summary report
	fmt.Printf("\n✅ %s\n", Header("DELETION SUMMARY"))
	fmt.Printf("📊 Lendings processed: %d\n", len(lendings))
	fmt.Printf("📊 Events deleted: %d (expected: %d)\n", deleted, expected)
	fmt.Printf("📈 Verification: %s\n", func() string {
		if deleted == expected {
			return BrightGreen("✓ Exactly 2 events per lending")
		}
		return BrightYellow(fmt.Sprintf("⚠ %d events deleted (expected %d)", deleted, expected))
	}())

	return nil
}

// clearFinishedLendingsSnapshots clears only the FinishedLendings snapshots after deletion operations.
func clearFinishedLendingsSnapshots(ctx context.Context, pool *pgxpool.Pool) error {
	// Clear snapshots for FinishedLendings queries (all variants with different limits/ordering)
	_, err := pool.Exec(ctx, "DELETE FROM snapshots WHERE projection_type LIKE 'FinishedLendings%'")
	return err
}

// clearCanceledReadersSnapshots clears only the CanceledReaders snapshots after deletion operations.
func clearCanceledReadersSnapshots(ctx context.Context, pool *pgxpool.Pool) error {
	// Clear snapshots for CanceledReaders queries (fixed name, no parameters)
	_, err := pool.Exec(ctx, "DELETE FROM snapshots WHERE projection_type = 'CanceledReaders'")
	return err
}

// clearRemovedBooksSnapshots clears only the RemovedBooks snapshots after deletion operations.
func clearRemovedBooksSnapshots(ctx context.Context, pool *pgxpool.Pool) error {
	// Clear snapshots for RemovedBooks queries (fixed name, no parameters)
	_, err := pool.Exec(ctx, "DELETE FROM snapshots WHERE projection_type = 'RemovedBooks'")
	return err
}

// initializePGXEventStore creates EventStore using pgx.Pool adapters with observability.
// Returns both the EventStore and the primary pgxpool.Pool for direct database access.
func initializePGXEventStore(ctx context.Context, cfg Config) (*postgresengine.EventStore, *pgxpool.Pool, error) {
	log.Printf("🔧 Initializing PGX adapter with connection pools")

	// Initialize primary connection.
	pgxPoolPrimary, primaryErr := pgxpool.NewWithConfig(ctx, config.PostgresPGXPoolPrimaryConfig())
	if primaryErr != nil {
		return nil, nil, fmt.Errorf("failed to create pgx pool for primary: %w", primaryErr)
	}

	// Test primary connection.
	if pingPrimaryErr := pgxPoolPrimary.Ping(ctx); pingPrimaryErr != nil {
		pgxPoolPrimary.Close()
		return nil, nil, fmt.Errorf("failed to connect to primary database: %w", pingPrimaryErr)
	}

	// Initialize replica connection.
	pgxPoolReplica, replicaErr := pgxpool.NewWithConfig(ctx, config.PostgresPGXPoolReplicaConfig())
	if replicaErr != nil {
		pgxPoolPrimary.Close()
		return nil, nil, fmt.Errorf("failed to create pgx pool for replica: %w", replicaErr)
	}

	// Test replica connection.
	if pingReplicaErr := pgxPoolReplica.Ping(ctx); pingReplicaErr != nil {
		pgxPoolPrimary.Close()
		pgxPoolReplica.Close()
		return nil, nil, fmt.Errorf("failed to connect to replica database: %w", pingReplicaErr)
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

	eventStore, err := postgresengine.NewEventStoreFromPGXPoolAndReplica(pgxPoolPrimary, pgxPoolReplica, eventStoreOptions...)
	if err != nil {
		pgxPoolPrimary.Close()
		pgxPoolReplica.Close()
		return nil, nil, fmt.Errorf("failed to create EventStore: %w", err)
	}

	return eventStore, pgxPoolPrimary, nil
}

// NewObservabilityConfig creates observability configuration for the cleanup tool.
func (c Config) NewObservabilityConfig() ObservabilityConfig {
	if !c.ObservabilityEnabled {
		return ObservabilityConfig{}
	}

	// Create real OpenTelemetry providers for the cleanup tool.
	_, err := config.NewObservabilityConfig()
	if err != nil {
		log.Printf("Failed to create observability providers: %v", err)
		return ObservabilityConfig{}
	}

	// Create real OpenTelemetry adapters.
	tracer := otel.Tracer("eventstore-cleanup-tool")
	meter := otel.Meter("eventstore-cleanup-tool")

	metricsCollector := oteladapters.NewMetricsCollector(meter)
	tracingCollector := oteladapters.NewTracingCollector(tracer)
	contextualLogger := oteladapters.NewSlogBridgeLogger("eventstore-cleanup-tool")

	return ObservabilityConfig{
		Logger:           nil, // Using contextual logger instead.
		ContextualLogger: contextualLogger,
		MetricsCollector: metricsCollector,
		TracingCollector: tracingCollector,
	}
}

// buildQueryOptions creates generic query options for observable wrappers.
func buildQueryOptions[Q shell.Query, R shell.QueryResult](obsConfig ObservabilityConfig) []observable.QueryOption[Q, R] {
	var opts []observable.QueryOption[Q, R]
	if obsConfig.MetricsCollector != nil {
		opts = append(opts, observable.WithQueryMetrics[Q, R](obsConfig.MetricsCollector))
	}
	if obsConfig.TracingCollector != nil {
		opts = append(opts, observable.WithQueryTracing[Q, R](obsConfig.TracingCollector))
	}
	if obsConfig.ContextualLogger != nil {
		opts = append(opts, observable.WithQueryContextualLogging[Q, R](obsConfig.ContextualLogger))
	}
	if obsConfig.Logger != nil {
		opts = append(opts, observable.WithQueryLogging[Q, R](obsConfig.Logger))
	}
	return opts
}

// buildRegisteredReadersQueryOptions creates query options for RegisteredReaders observable wrapper.
func buildRegisteredReadersQueryOptions(obsConfig ObservabilityConfig) []observable.QueryOption[registeredreaders.Query, registeredreaders.RegisteredReaders] {
	return buildQueryOptions[registeredreaders.Query, registeredreaders.RegisteredReaders](obsConfig)
}

// buildBooksLentOutQueryOptions creates query options for BooksLentOut observable wrapper.
func buildBooksLentOutQueryOptions(obsConfig ObservabilityConfig) []observable.QueryOption[bookslentout.Query, bookslentout.BooksLentOut] {
	return buildQueryOptions[bookslentout.Query, bookslentout.BooksLentOut](obsConfig)
}

// buildBooksInCirculationQueryOptions creates query options for BooksInCirculation observable wrapper.
func buildBooksInCirculationQueryOptions(obsConfig ObservabilityConfig) []observable.QueryOption[booksincirculation.Query, booksincirculation.BooksInCirculation] {
	return buildQueryOptions[booksincirculation.Query, booksincirculation.BooksInCirculation](obsConfig)
}

// performParallelCleanup processes orphaned lendings in parallel with 25 goroutines.
func performParallelCleanup(ctx context.Context, handler *returnbookcopyfromreader.CommandHandler, orphanedLendings []struct{ BookID, ReaderID string }, verbose bool) (processed, failed int) {
	stats := &ErrorStats{
		VerboseEnabled: verbose,
	}
	const numWorkers = 25

	// Create channels for work distribution
	jobs := make(chan struct{ BookID, ReaderID string }, len(orphanedLendings))
	results := make(chan bool, len(orphanedLendings))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for job := range jobs {
				select {
				case <-ctx.Done():
					results <- false // Context cancelled
					return
				default:
					// Process return command
					success := processReturnCommand(ctx, handler, job.BookID, job.ReaderID, stats)
					results <- success
				}
			}
		}()
	}

	// Send all jobs
	go func() {
		for _, lending := range orphanedLendings {
			jobs <- lending
		}
		close(jobs)
	}()

	// Wait for all workers to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	processed = 0
	failed = 0
	for success := range results {
		processed++
		if !success {
			failed++
		}

		// Progress logging every 50 operations
		if processed%50 == 0 {
			log.Printf("🔄 Progress: %d/%d processed (%.1f%%)",
				processed, len(orphanedLendings),
				float64(processed)*100.0/float64(len(orphanedLendings)))
		}
	}

	// Print error statistics
	stats.PrintSummary()

	return processed, failed
}

// processReturnCommand executes a single return command with error handling.
func processReturnCommand(ctx context.Context, handler *returnbookcopyfromreader.CommandHandler, bookID, readerID string, stats *ErrorStats) bool {
	bookUUID, err := uuid.Parse(bookID)
	if err != nil {
		stats.RecordError(fmt.Sprintf("Invalid bookID: %v", err), bookID, readerID)
		return false
	}

	readerUUID, err := uuid.Parse(readerID)
	if err != nil {
		stats.RecordError(fmt.Sprintf("Invalid readerID: %v", err), bookID, readerID)
		return false
	}

	command := returnbookcopyfromreader.BuildCommand(bookUUID, readerUUID, time.Now())

	commandCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	_, err = handler.Handle(commandCtx, command)
	if err != nil {
		stats.RecordError(err.Error(), bookID, readerID)
		return false
	}

	stats.RecordSuccess(bookID, readerID)
	return true
}

func parseFlags() Config {
	observability := flag.Bool("observability-enabled", false, "Enable OpenTelemetry observability")
	fixState := flag.Bool("fix-state", false, "Fix state inconsistencies (removes orphaned lendings and ghost books)")
	deleteCanceledReaders := flag.Bool("delete-canceled-readers", false, "Delete all events for canceled readers (safe ones only)")
	deleteRemovedBooks := flag.Bool("delete-removed-books", false, "Delete all events for removed books (safe ones only)")
	deleteFinishedLendings := flag.Bool("delete-finished-lendings", false, "Delete events for finished lendings")
	canceledReadersLimit := flag.Int("canceled-readers-limit", 1000, "Number of oldest canceled readers to delete")
	removedBooksLimit := flag.Int("removed-books-limit", 1000, "Number of oldest removed books to delete")
	finishedLendingsLimit := flag.Int("finished-lendings-limit", 500000, "Number of oldest finished lendings to query/delete, max = 500000")
	verbose := flag.Bool("verbose", false, "Show detailed breakdown with BookID and ReaderID for each error")
	flag.Parse()

	cfg := Config{
		ObservabilityEnabled:   *observability,
		PerformStateCleanup:    *fixState,
		DeleteCanceledReaders:  *deleteCanceledReaders,
		DeleteRemovedBooks:     *deleteRemovedBooks,
		DeleteFinishedLendings: *deleteFinishedLendings,
		CanceledReadersLimit:   *canceledReadersLimit,
		RemovedBooksLimit:      *removedBooksLimit,
		FinishedLendingsLimit:  *finishedLendingsLimit,
		VerboseOutput:          *verbose,
	}

	// Validate finished lendings limit bounds
	if cfg.FinishedLendingsLimit > 500000 {
		log.Panicf("❌ --finished-lendings-limit cannot exceed 500000 (requested %d)", cfg.FinishedLendingsLimit)
	}
	if cfg.FinishedLendingsLimit < 0 {
		log.Panicf("❌ --finished-lendings-limit cannot be negative (requested %d)", cfg.FinishedLendingsLimit)
	}

	// Validate canceled readers and removed books limits
	if cfg.CanceledReadersLimit < 0 {
		log.Panicf("❌ --canceled-readers-limit cannot be negative (requested %d)", cfg.CanceledReadersLimit)
	}
	if cfg.RemovedBooksLimit < 0 {
		log.Panicf("❌ --removed-books-limit cannot be negative (requested %d)", cfg.RemovedBooksLimit)
	}

	// Validate mutual exclusion - fix-state cannot be used with delete operations
	deleteFlagsCount := 0
	if cfg.DeleteCanceledReaders {
		deleteFlagsCount++
	}
	if cfg.DeleteRemovedBooks {
		deleteFlagsCount++
	}
	if cfg.DeleteFinishedLendings {
		deleteFlagsCount++
	}

	if cfg.PerformStateCleanup && deleteFlagsCount > 0 {
		log.Panicf("❌ Cannot use --fix-state with --delete-* flags. Choose either state cleanup OR event deletion.")
	}

	return cfg
}
