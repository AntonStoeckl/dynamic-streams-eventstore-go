package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine"
)

// =================================================================
// ACTOR SCHEDULER - Manages actor pools and batch processing
// =================================================================

// ActorScheduler manages the lifecycle and execution of all actors in the simulation.
// Key insight: Instead of 14,000 goroutines, we use pools and batch processing.
type ActorScheduler struct {
	// Context and control.
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	stopChan chan struct{}

	// Dependencies.
	eventStore *postgresengine.EventStore
	state      *SimulationState
	handlers   *HandlerBundle

	// Actor pools.
	activeReaders   []*ReaderActor    // Currently visiting the library (100-1000).
	inactiveReaders []*ReaderActor    // At home, not visiting today (~13,000+).
	librarians      []*LibrarianActor // Always active (2).

	// Pool management.
	mu                 sync.RWMutex
	currentActiveCount int
	targetActiveCount  int

	// Batch processing control.
	batchInProgress bool
	batchMutex      sync.Mutex

	// Statistics.
	stats SchedulerStats
}

// BatchResult contains timing information for batch operations.
type BatchResult struct {
	OperationCount     int
	OperationDurations []time.Duration
}

// SchedulerStats tracks scheduler performance metrics.
type SchedulerStats struct {
	TotalActorOperations int64
	ActiveReaderCount    int
	InactiveReaderCount  int
	BatchesProcessed     int64
	LastBatchDuration    time.Duration
	BatchesSkipped       int64         // Batches skipped due to overlap
	TotalBatchWaitTime   time.Duration // Time spent waiting between batches
}

// NewActorScheduler creates a new actor scheduler with initial populations.
func NewActorScheduler(ctx context.Context, eventStore *postgresengine.EventStore, state *SimulationState, handlers *HandlerBundle) (*ActorScheduler, error) {
	schedulerCtx, cancel := context.WithCancel(ctx)

	scheduler := &ActorScheduler{
		ctx:        schedulerCtx,
		cancel:     cancel,
		eventStore: eventStore,
		state:      state,
		handlers:   handlers,
		stopChan:   make(chan struct{}),

		activeReaders:   make([]*ReaderActor, 0, MaxActiveReaders),
		inactiveReaders: make([]*ReaderActor, 0, MaxReaders),
		librarians:      make([]*LibrarianActor, 0, LibrarianCount),

		targetActiveCount: InitialActiveReaders,
	}

	// Initialize actor populations.
	if err := scheduler.initializeActorPools(schedulerCtx); err != nil {
		cancel() // Clean up context
		return nil, fmt.Errorf("failed to initialize actor pools: %w", err)
	}

	return scheduler, nil
}

// initializeActorPools creates the initial reader and librarian populations.
//
//nolint:funlen
func (as *ActorScheduler) initializeActorPools(ctx context.Context) error {
	fmt.Printf("%s %s\n", StatusIcon("working"), Info("Initializing actor pools..."))

	// CRITICAL: Load all state from database FIRST before creating actors
	log.Printf("📚 Loading complete simulation state from EventStore...")
	if err := as.state.RefreshFromEventStore(ctx, as.handlers); err != nil {
		return fmt.Errorf("failed to load initial state from EventStore: %w", err)
	}

	stats := as.state.GetStats()
	fmt.Printf("%s %s %s %s, %s %s, %s %s\n",
		Success("✅"), BrightGreen("Complete state loaded:"),
		Bold(fmt.Sprintf("%d", stats.TotalBooks)), Info("total books"),
		Bold(fmt.Sprintf("%d", stats.AvailableBooks)), BrightGreen("available"),
		Bold(fmt.Sprintf("%d", stats.BooksLentOut)), BrightYellow("lent out"))

	// Create librarians (always active) based on LibrarianCount.
	librarianRoles := []LibrarianRole{Acquisitions, Maintenance}
	for i := 0; i < LibrarianCount; i++ {
		role := librarianRoles[i%len(librarianRoles)] // Cycle through roles if more than 2 librarians
		librarian := NewLibrarianActor(role)
		as.librarians = append(as.librarians, librarian)
	}

	// Get existing readers from already-loaded state (no database query needed)
	existingReaders := as.state.GetRegisteredReaders()
	totalExistingReaders := len(existingReaders)
	fmt.Printf("%s %s %s %s\n", StatusIcon("books"), Info("Found"), Bold(fmt.Sprintf("%d", totalExistingReaders)), Info("existing readers from loaded state"))

	// Create actors for existing readers
	actorsCreatedCount := 0
	for _, readerID := range existingReaders {
		persona := CasualReader
		if len(as.inactiveReaders) < totalExistingReaders/10 {
			persona = PowerUser // 10% power users
		}

		reader := NewReaderActor(persona)
		reader.ID = readerID // Use existing reader ID
		reader.Lifecycle = Registered

		as.inactiveReaders = append(as.inactiveReaders, reader)
		actorsCreatedCount++
	}

	// Create new readers if we need more to reach MinReaders
	readersToCreate := MinReaders - totalExistingReaders
	if readersToCreate > 0 {
		log.Printf("📚 Creating %d new readers to reach minimum...", readersToCreate)

		for i := 0; i < readersToCreate; i++ {
			persona := CasualReader
			if i < readersToCreate/10 {
				persona = PowerUser
			}

			reader := NewReaderActor(persona)
			reader.Lifecycle = Registered

			// Actually register this reader in the database
			if err := as.handlers.ExecuteRegisterReader(ctx, reader.ID); err != nil {
				log.Printf("⚠️  Failed to register reader %s: %v", reader.ID, err)
				continue
			}

			as.inactiveReaders = append(as.inactiveReaders, reader)
		}
	}

	// Critical: Populate ALL actor BorrowedBooks from already-loaded state (no database query needed)
	as.synchronizeActorBorrowedBooksFromLoadedState()

	// Move some readers to the active pool AFTER sync (so smart selection can work properly)
	as.adjustActiveReaderCount(as.targetActiveCount)

	// Show actual reader distribution after sync
	activeWithBooks := 0
	inactiveWithBooks := 0
	for _, reader := range as.activeReaders {
		if len(reader.BorrowedBooks) > 0 {
			activeWithBooks++
		}
	}
	for _, reader := range as.inactiveReaders {
		if len(reader.BorrowedBooks) > 0 {
			inactiveWithBooks++
		}
	}
	fmt.Printf("%s %s %s/%s %s, %s/%s %s\n", StatusIcon("books"), Info("Final reader distribution:"),
		Bold(fmt.Sprintf("%d", activeWithBooks)), fmt.Sprintf("%d", len(as.activeReaders)), BrightGreen("active have books"),
		Bold(fmt.Sprintf("%d", inactiveWithBooks)), fmt.Sprintf("%d", len(as.inactiveReaders)), BrightCyan("inactive have books"))

	// State already loaded at beginning - no need to reload

	as.currentActiveCount = len(as.activeReaders)
	log.Printf("👥 Actor pools initialized: %d active readers, %d inactive readers, %d librarians",
		len(as.activeReaders), len(as.inactiveReaders), len(as.librarians))

	return nil
}

// Start begins the actor scheduling and batch processing.
func (as *ActorScheduler) Start() error {
	log.Printf("%s %s", Success("🚀"), Success("Starting actor scheduler..."))

	// Start batch processing goroutines.
	as.wg.Add(5)
	go as.processReaderBatches()
	go as.processLibrarians()
	go as.managePopulation()
	go as.refreshState()
	go as.verifyLibrarianState()

	log.Printf("✅ Actor scheduler started with %d active readers", as.currentActiveCount)
	return nil
}

// Stop gracefully shuts down the actor scheduler.
func (as *ActorScheduler) Stop() error {
	log.Printf("🛑 Stopping actor scheduler...")

	close(as.stopChan)
	as.cancel()

	// Wait for all goroutines to finish.
	done := make(chan struct{})
	go func() {
		as.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Printf("✅ Actor scheduler stopped gracefully")
	case <-time.After(5 * time.Second):
		log.Printf("⚠️  Actor scheduler stop timeout")
	}

	return nil
}

// =================================================================
// BATCH PROCESSING - Core scheduling logic
// =================================================================

// processReaderBatches processes active readers sequentially to prevent pile-up.
// Each batch completes before the next one starts, with adaptive delays.
func (as *ActorScheduler) processReaderBatches() {
	defer as.wg.Done()

	for {
		select {
		case <-as.stopChan:
			return
		case <-as.ctx.Done():
			return
		default:
			// Check if a batch is already in progress
			as.batchMutex.Lock()
			if as.batchInProgress {
				// Skip this cycle if batch is still running
				as.stats.BatchesSkipped++
				as.batchMutex.Unlock()
				time.Sleep(50 * time.Millisecond) // Short wait before checking again
				continue
			}
			as.batchInProgress = true
			as.batchMutex.Unlock()

			// Process the batch and measure timing for adaptive delays
			batchStart := time.Now()
			as.processActiveReaders()
			batchDuration := time.Since(batchStart)

			// Mark batch as completed
			as.batchMutex.Lock()
			as.batchInProgress = false
			as.batchMutex.Unlock()

			// Adaptive delay: If batch completed quickly, wait the remainder
			minInterval := time.Duration(BatchProcessingDelayMs) * time.Millisecond
			if batchDuration < minInterval {
				remainingDelay := minInterval - batchDuration
				as.stats.TotalBatchWaitTime += remainingDelay
				time.Sleep(remainingDelay)
			}
			// If batch took longer than minimum interval, proceed immediately
		}
	}
}

// processActiveReaders processes all active readers in batches.
//
//nolint:funlen
func (as *ActorScheduler) processActiveReaders() {
	start := time.Now()

	// Create a timeout context for THIS batch round
	batchCtx, batchCancel := context.WithTimeout(as.ctx, 35*time.Second)
	defer batchCancel()

	as.mu.RLock()
	activeReaders := make([]*ReaderActor, len(as.activeReaders))
	copy(activeReaders, as.activeReaders)
	as.mu.RUnlock()

	if len(activeReaders) == 0 {
		return
	}

	// Get book statistics for start logging
	stateStats := as.state.GetStats()

	// Log batch start
	log.Printf("📊 Batch #%d started: %d readers | Books: %d total, %d lent out",
		as.stats.BatchesProcessed+1, len(activeReaders), stateStats.TotalBooks, stateStats.BooksLentOut)

	totalOperations := 0
	var allOperationDurations []time.Duration

	// Process readers in batches to control goroutine count.
	var batchWg sync.WaitGroup
	var resultsMu sync.Mutex

	for i := 0; i < len(activeReaders); i += ActorBatchSize {
		end := min(i+ActorBatchSize, len(activeReaders))
		batch := activeReaders[i:end]

		batchWg.Add(1)
		go func(readers []*ReaderActor, ctx context.Context) {
			defer batchWg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.Printf("🚨 PANIC in batch processing: %v", r)
				}
			}()

			// Pass batch context instead of as.ctx
			batchResult := as.processReaderBatch(ctx, readers)

			resultsMu.Lock()
			totalOperations += batchResult.OperationCount
			allOperationDurations = append(allOperationDurations, batchResult.OperationDurations...)
			resultsMu.Unlock()
		}(batch, batchCtx) // Pass batchCtx here
	}

	// Wait for all batches with timeout to prevent deadlock
	done := make(chan struct{})
	go func() {
		batchWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Normal completion
	case <-time.After(30 * time.Second):
		log.Printf("%s %s", CriticalError("🚨"), CriticalError(fmt.Sprintf("TIMEOUT: Batch processing exceeded 30s with %d readers - canceling batch", len(activeReaders))))
		batchCancel() // CRITICAL: Cancel the batch context!
		// This will cause all operations to fail with context.Canceled
	}

	// Clean up canceled readers from active pool
	as.cleanupCanceledReaders()

	// Update statistics.
	duration := time.Since(start)
	as.stats.LastBatchDuration = duration
	as.stats.BatchesProcessed++

	// Log every batch round for visibility during the normalization phase.
	{
		// Get book statistics from internal state
		stateStatsFinish := as.state.GetStats()

		// Check if book statistics changed during batch processing
		booksChanged := (stateStatsFinish.TotalBooks != stateStats.TotalBooks) ||
			(stateStatsFinish.BooksLentOut != stateStats.BooksLentOut)

		// Format message based on whether operations occurred
		if totalOperations > 0 {
			// Calculate actual average operation time from individual durations
			var totalOpTime time.Duration
			for _, d := range allOperationDurations {
				totalOpTime += d
			}
			avgOpTime := time.Duration(int64(totalOpTime) / int64(len(allOperationDurations)))

			throughput := float64(totalOperations) / duration.Seconds()

			// Add batch coordination info
			skippedInfo := ""
			if as.stats.BatchesSkipped > 0 {
				skippedInfo = fmt.Sprintf(" (skipped: %d)", as.stats.BatchesSkipped)
			}

			if booksChanged {
				log.Printf("📊 Batch #%d finished: %d ops in %v (avg: %v/op, %.1f ops/sec), %d readers%s | Books: %s total, %s lent out",
					as.stats.BatchesProcessed, totalOperations, duration.Round(time.Millisecond),
					avgOpTime.Round(time.Millisecond), throughput, len(activeReaders), skippedInfo,
					BrightYellow(fmt.Sprintf("%d", stateStatsFinish.TotalBooks)),
					BrightYellow(fmt.Sprintf("%d", stateStatsFinish.BooksLentOut)))
			} else {
				log.Printf("📊 Batch #%d finished: %d ops in %v (avg: %v/op, %.1f ops/sec), %d readers%s",
					as.stats.BatchesProcessed, totalOperations, duration.Round(time.Millisecond),
					avgOpTime.Round(time.Millisecond), throughput, len(activeReaders), skippedInfo)
			}
		} else {
			if booksChanged {
				log.Printf("📊 Batch #%d finished: %d operations in %v, %d readers | Books: %s total, %s lent out",
					as.stats.BatchesProcessed, totalOperations, duration.Round(time.Millisecond),
					len(activeReaders),
					BrightYellow(fmt.Sprintf("%d", stateStatsFinish.TotalBooks)),
					BrightYellow(fmt.Sprintf("%d", stateStatsFinish.BooksLentOut)))
			} else {
				log.Printf("📊 Batch #%d finished: %d operations in %v, %d readers",
					as.stats.BatchesProcessed, totalOperations, duration.Round(time.Millisecond),
					len(activeReaders))
			}
		}
	}
}

// processReaderBatch processes a single batch of readers.
func (as *ActorScheduler) processReaderBatch(ctx context.Context, readers []*ReaderActor) BatchResult {
	result := BatchResult{
		OperationDurations: make([]time.Duration, 0, len(readers)),
	}

	for _, reader := range readers {
		if ctx.Err() != nil { // Check batch context, not as.ctx
			return result // Context canceled or timeout
		}

		// Let reader decide if they want to do something.
		if reader.ShouldVisitLibrary() {
			// Track individual operation timing
			operationStart := time.Now()

			// Pass batch context, not as.ctx
			operationCount, err := reader.VisitLibrary(ctx, as.handlers)
			if err != nil {
				// Check if it was a timeout or cancellation
				if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
					log.Printf("⏱️ Reader %s timed out or canceled", reader.ID)
					return result // Stop processing this batch
				}
				// Log error but continue with other readers.
				log.Printf("⚠️  Reader %s library visit failed: %v", reader.ID, err)
			} else {
				// Record successful operation duration and count actual operations
				operationDuration := time.Since(operationStart)

				// Add one timing entry for each actual operation performed
				for i := 0; i < operationCount; i++ {
					result.OperationDurations = append(result.OperationDurations, operationDuration/time.Duration(operationCount))
				}

				result.OperationCount += operationCount
				as.stats.TotalActorOperations += int64(operationCount)
			}
		}

		// Check if the reader wants to cancel the contract.
		if reader.ShouldCancelContract() {
			as.handleReaderCancellation(reader)
		}
	}

	return result
}

// cleanupCanceledReaders removes canceled readers from the active pool.
func (as *ActorScheduler) cleanupCanceledReaders() {
	as.mu.Lock()
	defer as.mu.Unlock()

	// Filter out canceled readers from active pool
	activeReaders := make([]*ReaderActor, 0, len(as.activeReaders))
	for _, reader := range as.activeReaders {
		if reader.Lifecycle != Canceled {
			activeReaders = append(activeReaders, reader)
		}
	}

	// Update the active readers list
	removedCount := len(as.activeReaders) - len(activeReaders)
	as.activeReaders = activeReaders

	if removedCount > 0 {
		log.Printf("🧹 Cleaned up %d canceled readers from active pool", removedCount)
	}
}

// processLibrarians handles continuous librarian work.
func (as *ActorScheduler) processLibrarians() {
	defer as.wg.Done()

	ticker := time.NewTicker(time.Duration(BatchProcessingDelayMs*5) * time.Millisecond) // Slower than readers.
	defer ticker.Stop()

	for {
		select {
		case <-as.stopChan:
			return
		case <-as.ctx.Done():
			return
		case <-ticker.C:
			as.processLibrarianWork()
		}
	}
}

// processLibrarianWork executes librarian duties.
func (as *ActorScheduler) processLibrarianWork() {
	for _, librarian := range as.librarians {
		if as.ctx.Err() != nil {
			return
		}

		operationCount, err := librarian.Work(as.ctx, as.handlers, as.state)
		if err != nil {
			log.Printf("⚠️  Librarian %s work failed: %v", librarian.ID, err)
		}
		as.stats.TotalActorOperations += int64(operationCount)
	}
}

// verifyLibrarianState periodically queries the database to verify librarian state consistency.
// This runs independently from regular librarian work and uses the expensive query only for verification.
func (as *ActorScheduler) verifyLibrarianState() {
	defer as.wg.Done()

	ticker := time.NewTicker(60 * time.Second) // Every 60 seconds
	defer ticker.Stop()

	for {
		select {
		case <-as.stopChan:
			return
		case <-as.ctx.Done():
			return
		case <-ticker.C:
			// TEMPORARILY DISABLED: Expensive query causing 2-minute delays
			// as.performLibrarianStateVerification()
		}
	}
}

// performLibrarianStateVerification queries the database and compares with memory state.
// TEMPORARILY DISABLED: This expensive query (QueryBooksInCirculation) is taking 2+ minutes
// and causing significant load. Will be replaced with snapshotting/caching later.
//
//nolint:unused // Temporarily disabled but will be re-enabled with caching
func (as *ActorScheduler) performLibrarianStateVerification() {
	// Use extended timeout for this verification query
	ctx, cancel := context.WithTimeout(as.ctx, time.Duration(BooksInCirculationQueryTimeoutSeconds*float64(time.Second)))
	defer cancel()

	// Query actual database state
	booksResult, err := as.handlers.QueryBooksInCirculation(ctx)
	if err != nil {
		log.Printf("%s Librarian state verification failed: %s", SystemError("⚠️"), SystemError(err.Error()))
		return
	}

	// Compare with memory state
	memoryBooks := as.state.GetStats().TotalBooks
	databaseBooks := len(booksResult.Books)

	if memoryBooks != databaseBooks {
		log.Printf("📊 State verification: Memory=%d books, Database=%d books (diff=%d)",
			memoryBooks, databaseBooks, databaseBooks-memoryBooks)
	} else {
		log.Printf("✅ Librarian state verified: %d books match between memory and database", memoryBooks)
	}
}

// =================================================================
// STATE REFRESH - Periodic sync with EventStore truth
// =================================================================

// refreshState periodically refreshes in-memory state from the EventStore.
func (as *ActorScheduler) refreshState() {
	defer as.wg.Done()

	ticker := time.NewTicker(time.Duration(StateRefreshIntervalMs) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-as.stopChan:
			return
		case <-as.ctx.Done():
			return
		case <-ticker.C:
			if as.state.ShouldRefresh() {
				if err := as.state.RefreshFromEventStore(as.ctx, as.handlers); err != nil {
					log.Printf("⚠️  State refresh failed: %v", err)
				}
			}
		}
	}
}

// =================================================================
// POPULATION MANAGEMENT - Dynamic reader/book populations
// =================================================================

// managePopulation handles population dynamics (registration, cancellation, etc.).
func (as *ActorScheduler) managePopulation() {
	defer as.wg.Done()

	ticker := time.NewTicker(time.Duration(TuningIntervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-as.stopChan:
			return
		case <-as.ctx.Done():
			return
		case <-ticker.C:
			as.adjustPopulations()
		}
	}
}

// adjustPopulations manages reader registration/cancellation to maintain min/max.
func (as *ActorScheduler) adjustPopulations() {
	as.mu.Lock()
	defer as.mu.Unlock()

	totalReaders := len(as.activeReaders) + len(as.inactiveReaders)

	if totalReaders < MinReaders {
		// Need more readers - register new ones quickly.
		newReaders := min(10, MinReaders-totalReaders)
		for i := 0; i < newReaders; i++ {
			reader := NewReaderActor(CasualReader)
			reader.Lifecycle = Registered
			as.state.RegisterReader(reader.ID)
			as.inactiveReaders = append(as.inactiveReaders, reader)
		}
		log.Printf("📈 Registered %d new readers (total: %d)", newReaders, totalReaders+newReaders)

	} else if totalReaders > MaxReaders {
		// Too many readers - cancel some.
		readersToCancel := min(5, totalReaders-MaxReaders)
		as.cancelExcessReaders(readersToCancel)
		log.Printf("📉 Canceled %d readers (total: %d)", readersToCancel, totalReaders-readersToCancel)
	}
}

// handleReaderCancellation processes a reader wanting to cancel their contract.
func (as *ActorScheduler) handleReaderCancellation(reader *ReaderActor) {
	as.mu.Lock()
	defer as.mu.Unlock()

	// Only cancel if the reader has no borrowed books.
	borrowedBooks := as.state.GetReaderBorrowedBooks(reader.ID)
	if len(borrowedBooks) > 0 {
		return // Cannot cancel with outstanding loans.
	}

	// Execute the actual CancelReaderContract command
	if err := as.handlers.ExecuteCancelReader(as.ctx, reader.ID); err != nil {
		log.Printf("%s Failed to cancel reader contract %s: %s", BusinessError("⚠️"), reader.ID, BusinessError(err.Error()))
		return
	}

	// Mark as canceled but don't remove from active pool immediately
	// Let the batch processing finish, then remove during next adjustment
	reader.Lifecycle = Canceled

	// Update local state (will be corrected by state refresh)
	as.state.CancelReader(reader.ID)
}

// cancelExcessReaders removes readers when above maximum.
func (as *ActorScheduler) cancelExcessReaders(count int) {
	canceled := 0

	// Cancel from inactive readers first (they're not currently using the system).
	for i := len(as.inactiveReaders) - 1; i >= 0 && canceled < count; i-- {
		reader := as.inactiveReaders[i]

		// Only cancel readers with no borrowed books.
		borrowedBooks := as.state.GetReaderBorrowedBooks(reader.ID)
		if len(borrowedBooks) == 0 {
			// Execute the actual CancelReaderContract command
			if err := as.handlers.ExecuteCancelReader(as.ctx, reader.ID); err != nil {
				log.Printf("%s Failed to cancel reader contract %s: %s", BusinessError("⚠️"), reader.ID, BusinessError(err.Error()))
				continue
			}

			// Remove from inactive pool and update state
			as.inactiveReaders = append(as.inactiveReaders[:i], as.inactiveReaders[i+1:]...)
			as.state.CancelReader(reader.ID)
			reader.Lifecycle = Canceled
			canceled++
		}
	}
}

// =================================================================
// LOAD BALANCING - Active reader pool management
// =================================================================

// AdjustActiveReaderCount changes the number of active readers (called by load controller).
func (as *ActorScheduler) AdjustActiveReaderCount(newCount int) {
	as.mu.Lock()
	defer as.mu.Unlock()

	as.targetActiveCount = max(MinActiveReaders, min(newCount, MaxActiveReaders))
	as.adjustActiveReaderCount(as.targetActiveCount)
}

// adjustActiveReaderCount actually moves readers between active/inactive pools.
//
//nolint:gocognit,funlen // Complex pool management logic with smart selection requires length
func (as *ActorScheduler) adjustActiveReaderCount(targetCount int) {
	currentCount := len(as.activeReaders)

	if targetCount > currentCount {
		// Need more active readers.
		needed := targetCount - currentCount
		available := len(as.inactiveReaders)
		toActivate := min(needed, available)

		for i := 0; i < toActivate && len(as.inactiveReaders) > 0; i++ {
			// Smart reader selection: 50/50 strategy for encouraging returns vs. exploration
			preferReadersWithBooks := rand.Float32() < ChancePreferReadersWithBooks //nolint:gosec // Weak random OK for simulation

			var selectedReader *ReaderActor
			var randomIndex int

			if preferReadersWithBooks {
				// Build list of inactive readers with books
				readersWithBooks := make([]int, 0, len(as.inactiveReaders))
				for idx, reader := range as.inactiveReaders {
					if len(reader.BorrowedBooks) > 0 {
						readersWithBooks = append(readersWithBooks, idx)
					}
				}

				// If we found readers with books, select from them
				if len(readersWithBooks) > 0 {
					selectedIdx := readersWithBooks[rand.Intn(len(readersWithBooks))] //nolint:gosec // Weak random OK for simulation
					randomIndex = selectedIdx
					selectedReader = as.inactiveReaders[randomIndex]
				}
			}

			// Fallback to random selection if no preference or no readers with books
			if selectedReader == nil {
				randomIndex = rand.Intn(len(as.inactiveReaders)) //nolint:gosec // Weak random OK for simulation
				selectedReader = as.inactiveReaders[randomIndex]
			}

			// Move the selected reader from inactive to active
			as.inactiveReaders = append(as.inactiveReaders[:randomIndex], as.inactiveReaders[randomIndex+1:]...)
			as.activeReaders = append(as.activeReaders, selectedReader)
			selectedReader.Lifecycle = AtHome // Ready to visit.

			// Sync newly activated reader's borrowed books (10% chance for realistic business behavior metrics)
			if rand.Float64() < ChanceSyncOnActivation { //nolint:gosec // fine for the simulation
				if err := as.syncSingleReader(selectedReader); err != nil {
					log.Printf("⚠️ Failed to sync newly activated reader %s: %v", selectedReader.ID, err)
				}
			}
		}

		as.currentActiveCount = len(as.activeReaders)
		if toActivate > 0 {
			// Debug: Count readers with/without books in both pools
			activeWithBooks := 0
			inactiveWithBooks := 0
			for _, reader := range as.activeReaders {
				if len(reader.BorrowedBooks) > 0 {
					activeWithBooks++
				}
			}
			for _, reader := range as.inactiveReaders {
				if len(reader.BorrowedBooks) > 0 {
					inactiveWithBooks++
				}
			}

			// Get book statistics for context
			stateStats := as.state.GetStats()

			log.Printf("%s %s", AutoTune("📈"), AutoTune(fmt.Sprintf("Activated %d readers (%d -> %d active)",
				toActivate, currentCount, as.currentActiveCount)))
			log.Printf("📚 Reader distribution: %d/%d active have books, %d/%d inactive have books | Books: %d total, %d lent out",
				activeWithBooks, len(as.activeReaders), inactiveWithBooks, len(as.inactiveReaders), stateStats.TotalBooks, stateStats.BooksLentOut)
		}

	} else if targetCount < currentCount {
		// Need fewer active readers.
		excess := currentCount - targetCount

		for i := 0; i < excess; i++ {
			// Move the reader from active to inactive - FIFO (oldest first)
			reader := as.activeReaders[0]
			as.activeReaders = as.activeReaders[1:]
			as.inactiveReaders = append(as.inactiveReaders, reader)
			reader.Lifecycle = AtHome // At home, inactive.
		}

		as.currentActiveCount = len(as.activeReaders)
		log.Printf("📉 Deactivated %d readers (%d -> %d active)",
			excess, currentCount, as.currentActiveCount)
	}
}

// GetStats returns current scheduler statistics.
func (as *ActorScheduler) GetStats() SchedulerStats {
	as.mu.RLock()
	defer as.mu.RUnlock()

	stats := as.stats
	stats.ActiveReaderCount = len(as.activeReaders)
	stats.InactiveReaderCount = len(as.inactiveReaders)

	return stats
}

// =================================================================
// ACTOR STATE SYNCHRONIZATION
// =================================================================

// syncSingleReader synchronizes borrowed books for a single reader.
func (as *ActorScheduler) syncSingleReader(actor *ReaderActor) error {
	readerBooks, err := as.handlers.QueryBooksLentByReader(as.ctx, actor.ID)
	if err != nil {
		return fmt.Errorf("failed to query books for reader %s: %w", actor.ID, err)
	}

	// Convert to UUID slice
	borrowedBooks := make([]uuid.UUID, 0, len(readerBooks.Books))
	for _, book := range readerBooks.Books {
		bookID, err := uuid.Parse(book.BookID)
		if err != nil {
			log.Printf("⚠️ Invalid BookID '%s' for reader %s", book.BookID, actor.ID)
			continue // Skip invalid UUIDs
		}
		borrowedBooks = append(borrowedBooks, bookID)
	}

	// Update actor with borrowed books
	actor.BorrowedBooks = borrowedBooks
	return nil
}

// synchronizeActorBorrowedBooksFromLoadedState populates ALL actor BorrowedBooks using already-loaded state.
// This is more efficient than querying the database since state is already loaded.
func (as *ActorScheduler) synchronizeActorBorrowedBooksFromLoadedState() {
	// Get lending data from already-loaded simulation state
	stats := as.state.GetStats()
	totalLentBooks := stats.BooksLentOut

	// Access the reader-books mapping from state
	readerBooksMap := as.state.GetReaderBooksMap()

	// Populate actor BorrowedBooks from the loaded state
	totalActorsPopulated := 0
	totalBooksAssigned := 0

	allReaders := make([]*ReaderActor, 0, len(as.activeReaders)+len(as.inactiveReaders))
	allReaders = append(allReaders, as.activeReaders...)
	allReaders = append(allReaders, as.inactiveReaders...)
	for _, actor := range allReaders {
		if bookIDs, exists := readerBooksMap[actor.ID]; exists {
			actor.BorrowedBooks = make([]uuid.UUID, len(bookIDs))
			copy(actor.BorrowedBooks, bookIDs)
			totalBooksAssigned += len(bookIDs)
			totalActorsPopulated++
		}
	}

	fmt.Printf("🔗 %s %s %s %s %s (%s)\n",
		Info("Actor sync:"), Bold(fmt.Sprintf("%d", totalLentBooks)),
		BrightYellow("books lent out across"), Bold(fmt.Sprintf("%d", totalActorsPopulated)),
		BrightCyan("readers"), Gray("from loaded state"))
}
