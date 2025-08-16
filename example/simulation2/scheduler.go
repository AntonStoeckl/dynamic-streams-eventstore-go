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
	activeReaders   []*ReaderActor    // Currently visiting library (100-1000).
	inactiveReaders []*ReaderActor    // At home, not visiting today (~13,000+).
	librarians      []*LibrarianActor // Always active (2).

	// Pool management.
	mu                 sync.RWMutex
	currentActiveCount int
	targetActiveCount  int

	// Statistics.
	stats SchedulerStats
}

// SchedulerStats tracks scheduler performance metrics.
type SchedulerStats struct {
	TotalActorOperations int64
	ActiveReaderCount    int
	InactiveReaderCount  int
	BatchesProcessed     int64
	LastBatchDuration    time.Duration
}

// NewActorScheduler creates a new actor scheduler with initial populations.
func NewActorScheduler(ctx context.Context, eventStore *postgresengine.EventStore, state *SimulationState, handlers *HandlerBundle) *ActorScheduler {
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
	scheduler.initializeActorPools(schedulerCtx)

	return scheduler
}

// initializeActorPools creates the initial reader and librarian populations.
func (as *ActorScheduler) initializeActorPools(ctx context.Context) {
	log.Printf("🎭 Initializing actor pools...")

	// Create librarians (always active) based on LibrarianCount.
	librarianRoles := []LibrarianRole{Acquisitions, Maintenance}
	for i := 0; i < LibrarianCount; i++ {
		role := librarianRoles[i%len(librarianRoles)] // Cycle through roles if more than 2 librarians
		librarian := NewLibrarianActor(role)
		as.librarians = append(as.librarians, librarian)
	}

	// Query existing readers from the database first
	existingReaders, err := as.getExistingReaders(ctx)
	if err != nil {
		log.Printf("⚠️  Failed to query existing readers, starting fresh: %v", err)
		existingReaders = []uuid.UUID{}
	}

	totalExistingReaders := len(existingReaders)
	log.Printf("📚 Found %d existing readers in database", totalExistingReaders)

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

	// Move some readers to the active pool.
	as.adjustActiveReaderCount(as.targetActiveCount)

	// Critical: Populate ALL actor BorrowedBooks from database state using efficient BooksLentOut query
	if err := as.synchronizeAllActorBorrowedBooksFromLentOut(); err != nil {
		log.Printf("⚠️  Warning: Failed to synchronize actor borrowed books: %v", err)
	}

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
	log.Printf("📚 Reader distribution after sync: %d/%d active have books, %d/%d inactive have books",
		activeWithBooks, len(as.activeReaders), inactiveWithBooks, len(as.inactiveReaders))

	// CRITICAL: Load initial state before actors start working
	log.Printf("📚 Loading simulation state from EventStore...")
	if err := as.state.RefreshFromEventStore(ctx, as.handlers); err != nil {
		log.Printf("⚠️  Warning: Failed to load simulation state: %v", err)
	} else {
		stats := as.state.GetStats()
		log.Printf("✅ Simulation state loaded: %d total books, %d available, %d lent out",
			stats.TotalBooks, stats.AvailableBooks, stats.BooksLentOut)
	}

	as.currentActiveCount = len(as.activeReaders)
	log.Printf("👥 Actor pools initialized: %d active readers, %d inactive readers, %d librarians",
		len(as.activeReaders), len(as.inactiveReaders), len(as.librarians))
}

// Start begins the actor scheduling and batch processing.
func (as *ActorScheduler) Start() error {
	log.Printf("🚀 Starting actor scheduler...")

	// Start batch processing goroutines.
	as.wg.Add(4)
	go as.processReaderBatches()
	go as.processLibrarians()
	go as.managePopulation()
	go as.refreshState()

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

// processReaderBatches processes active readers in batches to avoid goroutine explosion.
func (as *ActorScheduler) processReaderBatches() {
	defer as.wg.Done()

	ticker := time.NewTicker(time.Duration(BatchProcessingDelayMs) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-as.stopChan:
			return
		case <-as.ctx.Done():
			return
		case <-ticker.C:
			as.processActiveReaders()
		}
	}
}

// processActiveReaders processes all active readers in batches.
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

	totalOperations := 0

	// Process readers in batches to control goroutine count.
	var batchWg sync.WaitGroup
	var operationsMu sync.Mutex

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
			batchOps := as.processReaderBatch(ctx, readers)

			operationsMu.Lock()
			totalOperations += batchOps
			operationsMu.Unlock()
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
		log.Printf("🚨 TIMEOUT: Batch processing exceeded 30s with %d readers - cancelling batch", len(activeReaders))
		batchCancel() // CRITICAL: Cancel the batch context!
		// This will cause all operations to fail with context.Canceled
	}

	// Clean up cancelled readers from active pool
	as.cleanupCancelledReaders()

	// Update statistics.
	duration := time.Since(start)
	as.stats.LastBatchDuration = duration
	as.stats.BatchesProcessed++

	// Log every batch round for visibility during normalization phase.
	{
		// Calculate average time per operation for clarity
		avgOpTime := "N/A"
		if totalOperations > 0 {
			avgOpTime = fmt.Sprintf("%.0fms", float64(duration.Milliseconds())/float64(totalOperations))
		}

		// Get book statistics from internal state
		stateStats := as.state.GetStats()

		log.Printf("📊 Batch round #%d: %d operations in %v total (avg: %s/op with overhead), %d active readers | Books: %d total, %d lent out",
			as.stats.BatchesProcessed, totalOperations, duration, avgOpTime, len(activeReaders), stateStats.TotalBooks, stateStats.BooksLentOut)
	}
}

// processReaderBatch processes a single batch of readers.
func (as *ActorScheduler) processReaderBatch(ctx context.Context, readers []*ReaderActor) int {
	operationsThisBatch := 0
	for _, reader := range readers {
		if ctx.Err() != nil { // Check batch context, not as.ctx
			return operationsThisBatch // Context cancelled or timeout
		}

		// Let reader decide if they want to do something.
		if reader.ShouldVisitLibrary() {
			// Pass batch context, not as.ctx
			if err := reader.VisitLibrary(ctx, as.handlers); err != nil {
				// Check if it was a timeout or cancellation
				if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
					log.Printf("⏱️ Reader %s timed out or cancelled", reader.ID)
					return operationsThisBatch // Stop processing this batch
				}
				// Log error but continue with other readers.
				log.Printf("⚠️  Reader %s library visit failed: %v", reader.ID, err)
			} else {
				// Only count successful visits as operations
				operationsThisBatch++
				as.stats.TotalActorOperations++
			}
		}

		// Check if the reader wants to cancel the contract.
		if reader.ShouldCancelContract() {
			as.handleReaderCancellation(reader)
		}
	}

	return operationsThisBatch
}

// cleanupCancelledReaders removes cancelled readers from the active pool.
func (as *ActorScheduler) cleanupCancelledReaders() {
	as.mu.Lock()
	defer as.mu.Unlock()

	// Filter out cancelled readers from active pool
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
		log.Printf("🧹 Cleaned up %d cancelled readers from active pool", removedCount)
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

		if err := librarian.Work(as.ctx, as.handlers); err != nil {
			log.Printf("⚠️  Librarian %s work failed: %v", librarian.ID, err)
		}
		as.stats.TotalActorOperations++
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
		log.Printf("📉 Cancelled %d readers (total: %d)", readersToCancel, totalReaders-readersToCancel)
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
		log.Printf("⚠️  Failed to cancel reader contract %s: %v", reader.ID, err)
		return
	}

	// Mark as cancelled but don't remove from active pool immediately
	// Let the batch processing finish, then remove during next adjustment
	reader.Lifecycle = Canceled

	// Update local state (will be corrected by state refresh)
	as.state.CancelReader(reader.ID)
}

// cancelExcessReaders removes readers when above maximum.
func (as *ActorScheduler) cancelExcessReaders(count int) {
	cancelled := 0

	// Cancel from inactive readers first (they're not currently using the system).
	for i := len(as.inactiveReaders) - 1; i >= 0 && cancelled < count; i-- {
		reader := as.inactiveReaders[i]

		// Only cancel readers with no borrowed books.
		borrowedBooks := as.state.GetReaderBorrowedBooks(reader.ID)
		if len(borrowedBooks) == 0 {
			// Execute the actual CancelReaderContract command
			if err := as.handlers.ExecuteCancelReader(as.ctx, reader.ID); err != nil {
				log.Printf("⚠️  Failed to cancel reader contract %s: %v", reader.ID, err)
				continue
			}

			// Remove from inactive pool and update state
			as.inactiveReaders = append(as.inactiveReaders[:i], as.inactiveReaders[i+1:]...)
			as.state.CancelReader(reader.ID)
			reader.Lifecycle = Canceled
			cancelled++
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
func (as *ActorScheduler) adjustActiveReaderCount(targetCount int) {
	currentCount := len(as.activeReaders)

	if targetCount > currentCount {
		// Need more active readers.
		needed := targetCount - currentCount
		available := len(as.inactiveReaders)
		toActivate := min(needed, available)

		for i := 0; i < toActivate && len(as.inactiveReaders) > 0; i++ {
			// Simple random selection - let natural probabilities handle behavior
			randomIndex := rand.Intn(len(as.inactiveReaders)) //nolint:gosec // Weak random OK for simulation
			selectedReader := as.inactiveReaders[randomIndex]

			// Move the selected reader from inactive to active
			as.inactiveReaders = append(as.inactiveReaders[:randomIndex], as.inactiveReaders[randomIndex+1:]...)
			as.activeReaders = append(as.activeReaders, selectedReader)
			selectedReader.Lifecycle = AtHome // Ready to visit.

			// Sync newly activated reader's borrowed books
			if err := as.syncSingleReader(selectedReader); err != nil {
				log.Printf("⚠️ Failed to sync newly activated reader %s: %v", selectedReader.ID, err)
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

			log.Printf("📈 Activated %d readers (%d -> %d active)",
				toActivate, currentCount, as.currentActiveCount)
			log.Printf("📚 Reader distribution: %d/%d active have books, %d/%d inactive have books | Books: %d total, %d lent out",
				activeWithBooks, len(as.activeReaders), inactiveWithBooks, len(as.inactiveReaders), stateStats.TotalBooks, stateStats.BooksLentOut)
		}

	} else if targetCount < currentCount {
		// Need fewer active readers.
		excess := currentCount - targetCount

		for i := 0; i < excess; i++ {
			// Move the reader from active to inactive.
			reader := as.activeReaders[len(as.activeReaders)-1]
			as.activeReaders = as.activeReaders[:len(as.activeReaders)-1]
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
			continue // Skip invalid UUIDs
		}
		borrowedBooks = append(borrowedBooks, bookID)
	}

	// Update actor with borrowed books
	actor.BorrowedBooks = borrowedBooks
	return nil
}

// synchronizeActorBorrowedBooks populates actor BorrowedBooks from database state.
// Only syncs active readers to avoid performance issues.
func (as *ActorScheduler) synchronizeActorBorrowedBooks() error {
	// Take snapshot of active readers under lock to prevent race condition
	as.mu.RLock()
	activeSnapshot := make([]*ReaderActor, len(as.activeReaders))
	copy(activeSnapshot, as.activeReaders)
	as.mu.RUnlock()

	syncCount := 0
	readersWithBooks := 0

	// Process active readers snapshot (no inactive readers for performance)
	for _, actor := range activeSnapshot {
		if err := as.syncSingleReader(actor); err != nil {
			log.Printf("⚠️ Failed to sync reader %s: %v", actor.ID, err)
			continue
		}
		if len(actor.BorrowedBooks) > 0 {
			syncCount += len(actor.BorrowedBooks)
			readersWithBooks++
		}
	}

	if syncCount > 0 {
		log.Printf("🔗 Synchronized %d borrowed books across %d active readers (targeted queries)",
			syncCount, readersWithBooks)
	}

	return nil
}

// synchronizeAllActorBorrowedBooksFromLentOut populates ALL actor BorrowedBooks using single BooksLentOut query.
// Used only at startup for efficiency (one query vs thousands).
func (as *ActorScheduler) synchronizeAllActorBorrowedBooksFromLentOut() error {
	// Query all lending relationships at once
	lentBooksResult, err := as.handlers.QueryBooksLentOut(as.ctx)
	if err != nil {
		return fmt.Errorf("failed to query books lent out: %w", err)
	}

	// Create map for fast lookup: readerID -> []bookID
	readerBooksMap := make(map[uuid.UUID][]uuid.UUID)

	// Track unique books to detect duplicates
	uniqueBooks := make(map[uuid.UUID]bool)

	for _, lending := range lentBooksResult.Lendings {
		readerID, err := uuid.Parse(lending.ReaderID)
		if err != nil {
			continue // Skip invalid UUIDs
		}

		bookID, err := uuid.Parse(lending.BookID)
		if err != nil {
			continue // Skip invalid UUIDs
		}

		// Check for duplicate books
		if uniqueBooks[bookID] {
			log.Printf("⚠️  DUPLICATE BOOK FOUND: %s is lent to multiple readers!", bookID)
		}
		uniqueBooks[bookID] = true

		if readerBooksMap[readerID] == nil {
			readerBooksMap[readerID] = make([]uuid.UUID, 0)
		}
		readerBooksMap[readerID] = append(readerBooksMap[readerID], bookID)
	}

	// Take snapshots to avoid race condition
	as.mu.RLock()
	activeSnapshot := make([]*ReaderActor, len(as.activeReaders))
	copy(activeSnapshot, as.activeReaders)
	inactiveSnapshot := make([]*ReaderActor, len(as.inactiveReaders))
	copy(inactiveSnapshot, as.inactiveReaders)
	as.mu.RUnlock()

	syncCount := 0
	readersWithBooks := 0
	missingReaders := 0

	// Update all active readers
	for _, actor := range activeSnapshot {
		if borrowedBooks, exists := readerBooksMap[actor.ID]; exists {
			actor.BorrowedBooks = make([]uuid.UUID, len(borrowedBooks))
			copy(actor.BorrowedBooks, borrowedBooks)
			syncCount += len(borrowedBooks)
			readersWithBooks++
		} else {
			actor.BorrowedBooks = nil // Ensure empty if no books
		}
	}

	// Update all inactive readers
	for _, actor := range inactiveSnapshot {
		if borrowedBooks, exists := readerBooksMap[actor.ID]; exists {
			actor.BorrowedBooks = make([]uuid.UUID, len(borrowedBooks))
			copy(actor.BorrowedBooks, borrowedBooks)
			syncCount += len(borrowedBooks)
			readersWithBooks++
		} else {
			actor.BorrowedBooks = nil // Ensure empty if no books
		}
	}

	// Debug: Check for readers in lendings but not in actors
	for readerID := range readerBooksMap {
		found := false
		// Check active actors
		for _, actor := range activeSnapshot {
			if actor.ID == readerID {
				found = true
				break
			}
		}
		// Check inactive actors if not found in active
		if !found {
			for _, actor := range inactiveSnapshot {
				if actor.ID == readerID {
					found = true
					break
				}
			}
		}
		if !found {
			missingReaders++
		}
	}

	log.Printf("🔗 Actor sync: %d books lent out across %d readers (BooksLentOut query)",
		syncCount, readersWithBooks)

	if missingReaders > 0 {
		booksFromMissingReaders := 0
		for readerID, books := range readerBooksMap {
			found := false
			// Quick check if this reader exists in snapshots
			for _, actor := range activeSnapshot {
				if actor.ID == readerID {
					found = true
					break
				}
			}
			if !found {
				for _, actor := range inactiveSnapshot {
					if actor.ID == readerID {
						found = true
						break
					}
				}
			}
			if !found {
				booksFromMissingReaders += len(books)
			}
		}
	}

	return nil
}

// =================================================================
// HELPER FUNCTIONS
// =================================================================

// getExistingReaders queries the database for existing registered readers.
func (as *ActorScheduler) getExistingReaders(ctx context.Context) ([]uuid.UUID, error) {
	readersResult, err := as.handlers.QueryRegisteredReaders(ctx)
	if err != nil {
		return nil, err
	}

	readerIDs := make([]uuid.UUID, 0, len(readersResult.Readers))
	skippedReaders := 0
	for _, reader := range readersResult.Readers {
		readerID, parseErr := uuid.Parse(reader.ReaderID)
		if parseErr != nil {
			skippedReaders++
			log.Printf("⚠️  Skipping reader with invalid UUID: %s (error: %v)", reader.ReaderID, parseErr)
			continue // Skip invalid UUIDs
		}
		readerIDs = append(readerIDs, readerID)
	}

	return readerIDs, nil
}
