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
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/query/registeredreaders"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell/config"
)

// Config holds the command-line configuration for the cleanup tool.
type Config struct {
	ObservabilityEnabled bool
	PerformCleanup       bool
	VerboseOutput        bool
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

//nolint:gocognit,gocyclo,funlen // Main function orchestrates entire cleanup workflow - complexity justified for a single-purpose tool
func main() {
	fmt.Printf("%s %s\n", StatusIcon("debug"), Header("Data Consistency Analysis Tool"))
	fmt.Printf("%s\n", Separator("=", 33))

	cfg := parseFlags()

	ctx := context.Background()

	// Initialize EventStore (same as simulation)
	eventStore, err := initializePGXEventStore(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize EventStore: %v", err)
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

	registeredReadersHandler, err := registeredreaders.NewQueryHandler(eventStore,
		buildRegisteredReadersOptions(obsConfig)...)
	if err != nil {
		log.Fatalf("Failed to create RegisteredReaders handler: %v", err)
	}

	booksLentOutHandler, err := bookslentout.NewQueryHandler(eventStore,
		buildBooksLentOutOptions(obsConfig)...)
	if err != nil {
		log.Fatalf("Failed to create BooksLentOut handler: %v", err)
	}

	booksInCirculationHandler, err := booksincirculation.NewQueryHandler(eventStore,
		buildBooksInCirculationOptions(obsConfig)...)
	if err != nil {
		log.Fatalf("Failed to create BooksInCirculation handler: %v", err)
	}

	// Query all data
	fmt.Printf("%s %s\n", StatusIcon("stats"), Info("Querying database state..."))

	// 1. Get all registered readers
	registeredReadersResult, err := registeredReadersHandler.Handle(ctx)
	if err != nil {
		log.Fatalf("Failed to query registered readers: %v", err)
	}

	// 2. Get all books lent out
	booksLentOutResult, err := booksLentOutHandler.Handle(ctx)
	if err != nil {
		log.Fatalf("Failed to query books lent out: %v", err)
	}

	// 3. Get all books in circulation
	booksInCirculationResult, err := booksInCirculationHandler.Handle(ctx)
	if err != nil {
		log.Fatalf("Failed to query books in circulation: %v", err)
	}

	// Analysis
	fmt.Printf("\n%s %s\n", StatusIcon("stats"), Header("ANALYSIS RESULTS"))
	fmt.Printf("%s\n", Separator("=", 18))

	// Basic counts
	fmt.Printf("%s %s %s\n", StatusIcon("readers"), BrightCyan("Total registered readers:"), Bold(fmt.Sprintf("%d", len(registeredReadersResult.Readers))))
	fmt.Printf("%s %s %s\n", StatusIcon("books"), BrightCyan("Total books in circulation:"), Bold(fmt.Sprintf("%d", len(booksInCirculationResult.Books))))
	fmt.Printf("🔗 %s %s\n", BrightCyan("Total open lendings:"), Bold(fmt.Sprintf("%d", len(booksLentOutResult.Lendings))))

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
	if !cfg.PerformCleanup {
		log.Printf("💡 To perform cleanup, run with --cleanup flag")
		log.Printf("💡 Example: ./cleanup --cleanup --observability-enabled")
		log.Printf("💡 For detailed output: ./cleanup --cleanup --verbose")
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
	returnBookHandler, err := returnbookcopyfromreader.NewCommandHandler(eventStore,
		buildReturnBookOptions(obsConfig)...)
	if err != nil {
		log.Fatalf("Failed to create ReturnBookCopyFromReader handler: %v", err)
	}

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

	// Process cleanup in parallel with 10 goroutines
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

	// Process ghost cleanup in parallel with 10 goroutines
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

// buildRegisteredReadersOptions creates options for RegisteredReaders query handler.
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

// buildBooksLentOutOptions creates options for BooksLentOut query handler.
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

// buildBooksInCirculationOptions creates options for BooksInCirculation query handler.
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

// performParallelCleanup processes orphaned lendings in parallel with 10 goroutines.
func performParallelCleanup(ctx context.Context, handler *returnbookcopyfromreader.CommandHandler, orphanedLendings []struct{ BookID, ReaderID string }, verbose bool) (processed, failed int) {
	stats := &ErrorStats{
		VerboseEnabled: verbose,
	}
	const numWorkers = 10

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

	err = handler.Handle(commandCtx, command)
	if err != nil {
		stats.RecordError(err.Error(), bookID, readerID)
		return false
	}

	stats.RecordSuccess(bookID, readerID)
	return true
}

// buildReturnBookOptions creates options for ReturnBookCopyFromReader command handler.
func buildReturnBookOptions(obsConfig ObservabilityConfig) []returnbookcopyfromreader.Option {
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

func parseFlags() Config {
	observability := flag.Bool("observability-enabled", false, "Enable OpenTelemetry observability")
	cleanup := flag.Bool("cleanup", false, "Actually perform cleanup operations (removes orphaned lendings and ghost books)")
	verbose := flag.Bool("verbose", false, "Show detailed breakdown with BookID and ReaderID for each error")
	flag.Parse()

	return Config{
		ObservabilityEnabled: *observability,
		PerformCleanup:       *cleanup,
		VerboseOutput:        *verbose,
	}
}
