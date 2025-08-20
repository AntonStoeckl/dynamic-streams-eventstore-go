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
// UNIFIED SIMULATION - Single loop for batch processing and auto-tuning
// =================================================================

// UnifiedSimulation replaces the complex scheduler/controller architecture
// with a single, sequential loop that provides immediate feedback.
type UnifiedSimulation struct {
	// Context and control
	ctx      context.Context
	cancel   context.CancelFunc
	stopChan chan struct{}

	// Dependencies
	eventStore *postgresengine.EventStore
	state      *SimulationState
	handlers   *HandlerBundle

	// Actor pools (no locks needed - single threaded!)
	activeReaders   []*ReaderActor
	inactiveReaders []*ReaderActor
	librarians      []*LibrarianActor

	// Metrics
	recentBatches   []BatchMetrics
	maxBatchHistory int

	// Auto-tuning state
	targetActiveCount        int
	consecutiveGoodCycles    int
	consecutiveBadCycles     int
	lastPopulationAdjustment time.Time

	// Statistics
	stats UnifiedStats
}

// BatchMetrics contains performance data from a single batch.
type BatchMetrics struct {
	OperationCount     int
	OperationLatencies []time.Duration
	TimeoutCount       int
	BatchDuration      time.Duration
	Timestamp          time.Time
}

// batchResult holds metrics from a concurrent reader group.
type batchResult struct {
	operationCount     int
	operationLatencies []time.Duration
	timeoutCount       int
	readersProcessed   int
}

// UnifiedStats tracks overall simulation performance.
type UnifiedStats struct {
	TotalCycles                int64
	TotalOperations            int64
	AutoTuneAdjustments        int64
	ScaleUpEvents              int64
	ScaleDownEvents            int64
	CurrentActiveReaders       int
	CurrentAvgLatencyMs        int64
	CurrentThroughputOpsPerSec float64
	TimeoutRate                float64
	LastAdjustmentReason       string
}

// NewUnifiedSimulation creates a new unified simulation.
func NewUnifiedSimulation(
	ctx context.Context,
	eventStore *postgresengine.EventStore,
	state *SimulationState,
	handlers *HandlerBundle,
) (*UnifiedSimulation, error) {
	sim := &UnifiedSimulation{
		ctx:        ctx,
		cancel:     func() {}, // Managed by caller
		eventStore: eventStore,
		state:      state,
		handlers:   handlers,
		stopChan:   make(chan struct{}),

		activeReaders:   make([]*ReaderActor, 0, MaxActiveReaders),
		inactiveReaders: make([]*ReaderActor, 0, MaxReaders),
		librarians:      make([]*LibrarianActor, 0, LibrarianCount),

		recentBatches:            make([]BatchMetrics, 0, 10),
		maxBatchHistory:          10,
		targetActiveCount:        InitialActiveReaders,
		lastPopulationAdjustment: time.Now(),
	}

	// Initialize actor pools
	if err := sim.initializeActorPools(); err != nil {
		return nil, fmt.Errorf("failed to initialize actor pools: %w", err)
	}

	return sim, nil
}

// initializeActorPools creates the initial reader and librarian populations
//
//nolint:funlen
func (s *UnifiedSimulation) initializeActorPools() error {
	log.Printf("%s %s", StatusIcon("working"), Info("Initializing actor pools..."))

	// Load all state from the database first
	log.Printf("📚 Loading complete simulation state from EventStore...")
	if err := s.state.InitializeStateFromEventStore(s.ctx, s.handlers); err != nil {
		return fmt.Errorf("failed to load initial state from EventStore: %w", err)
	}

	stats := s.state.GetStats()
	fmt.Printf("%s %s %s %s, %s %s, %s %s\n",
		Success("✅"), BrightGreen("Complete state loaded:"),
		Bold(fmt.Sprintf("%d", stats.TotalBooks)), Info("total books"),
		Bold(fmt.Sprintf("%d", stats.AvailableBooks)), BrightGreen("available"),
		Bold(fmt.Sprintf("%d", stats.BooksLentOut)), BrightYellow("lent out"))

	// Create librarians
	librarianRoles := []LibrarianRole{Acquisitions, Maintenance}
	for i := 0; i < LibrarianCount; i++ {
		role := librarianRoles[i%len(librarianRoles)]
		librarian := NewLibrarianActor(role)
		s.librarians = append(s.librarians, librarian)
	}

	// Get existing readers from the loaded state
	existingReaders := s.state.GetRegisteredReaders()
	totalExistingReaders := len(existingReaders)
	fmt.Printf("%s %s %s %s\n", StatusIcon("books"), Info("Found"), Bold(fmt.Sprintf("%d", totalExistingReaders)), Info("existing readers from loaded state"))

	// Create actors for existing readers
	for _, readerID := range existingReaders {
		persona := CasualReader
		if len(s.inactiveReaders) < totalExistingReaders/10 {
			persona = PowerUser // 10% power users
		}

		reader := NewReaderActor(persona)
		reader.ID = readerID
		reader.Lifecycle = Registered

		s.inactiveReaders = append(s.inactiveReaders, reader)
	}

	// Create new readers if needed
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

			if err := s.handlers.ExecuteRegisterReader(s.ctx, reader.ID); err != nil {
				log.Printf("⚠️  Failed to register reader %s: %v", reader.ID, err)
				continue
			}

			s.inactiveReaders = append(s.inactiveReaders, reader)
		}
	}

	// Populate actor BorrowedBooks from the loaded state
	s.synchronizeActorBorrowedBooksFromLoadedState()

	// Move some readers to the active pool
	s.adjustActiveReaderCount(s.targetActiveCount)

	log.Printf("👥 Actor pools initialized: %d active readers, %d inactive readers, %d librarians",
		len(s.activeReaders), len(s.inactiveReaders), len(s.librarians))

	return nil
}

// Start begins the unified simulation loop.
func (s *UnifiedSimulation) Start(cfg Config) error {
	log.Printf("%s %s", Success("🚀"), Success("Starting unified simulation..."))
	log.Printf("🎯 Auto-tune targets: avg<%dms/op, timeouts<%.1f%%",
		TargetAvgLatencyMs, MaxTimeoutRate*100)

	// Run the main simulation loop
	return s.runMainLoop(cfg)
}

// Stop gracefully shuts down the simulation.
func (s *UnifiedSimulation) Stop() error {
	log.Printf("🛑 Stopping unified simulation...")

	close(s.stopChan)
	// Note: Caller manages context cancellation

	log.Printf("✅ Unified simulation stopped")
	return nil
}

// =================================================================
// MAIN SIMULATION LOOP - The heart of the simplified architecture
// =================================================================

// runMainLoop is the single, sequential loop that handles everything.
func (s *UnifiedSimulation) runMainLoop(cfg Config) error {
	cycleNum := int64(0)
	log.Printf("%s %s", AutoTune("🔄"), AutoTune("Main simulation loop started"))

	for {
		select {
		case <-s.stopChan:
			log.Printf("🛑 Simulation received stop signal")
			return nil
		case <-s.ctx.Done():
			log.Printf("🛑 Simulation context canceled - shutting down gracefully")
			return nil
		default:
			cycleStart := time.Now()

			// Every batch: Core processing
			s.rotateActiveReaders()
			batchMetrics := s.processBatch(cfg, cycleNum)
			s.recordBatchMetrics(batchMetrics)

			// Every batch: Auto-tuning (immediate feedback)
			s.autoTuneFromRecentMetrics()

			// Every 5 batches: Do Librarian work
			if cycleNum%5 == 0 {
				s.processLibrarians()
			}

			// Every 10 batches: Population management
			if cycleNum%10 == 0 {
				s.adjustPopulationIfNeeded()
			}

			// Update stats
			s.stats.TotalCycles = cycleNum
			cycleNum++

			// Log cycle timing for visibility
			cycleDuration := time.Since(cycleStart)
			log.Printf("📊 Cycle #%d completed in %v", cycleNum, cycleDuration.Round(time.Millisecond))

			s.logPerformance()
		}
	}
}

// =================================================================
// BATCH PROCESSING - Core work in each cycle
// =================================================================

// processBatch processes all active readers using a worker pool pattern.
func (s *UnifiedSimulation) processBatch(cfg Config, cycleNum int64) BatchMetrics {
	batchStart := time.Now()

	if len(s.activeReaders) == 0 {
		return BatchMetrics{Timestamp: batchStart}
	}

	// Create batch context with timeout (using helper for metadata)
	batchCtx, batchCancel := WithBatchTimeout(s.ctx, BatchTimeoutSeconds*time.Second)
	defer batchCancel()

	log.Printf("📊 Cycle #%d started", cycleNum+1)

	// Execute worker pool and collect results
	totalOperations, allLatencies, timeoutCount, batchTimedOut, numWorkers := s.executeWorkerPool(batchCtx, cfg, cycleNum)

	batchDuration := time.Since(batchStart)

	// Log batch completion
	s.logBatchCompletion(cycleNum, totalOperations, batchDuration, timeoutCount, batchTimedOut, numWorkers)

	return BatchMetrics{
		OperationCount:     totalOperations,
		OperationLatencies: allLatencies,
		TimeoutCount:       timeoutCount,
		BatchDuration:      batchDuration,
		Timestamp:          batchStart,
	}
}

// executeWorkerPool sets up worker pool, processes readers, and collects results.
func (s *UnifiedSimulation) executeWorkerPool(batchCtx context.Context, cfg Config, cycleNum int64) (int, []time.Duration, int, bool, int) {
	// Create channels for work distribution (no locks needed!)
	numWorkers := cfg.Workers

	if numWorkers > len(s.activeReaders) {
		numWorkers = len(s.activeReaders)
	}

	readerChan := make(chan *ReaderActor, len(s.activeReaders))
	resultChan := make(chan batchResult, numWorkers)

	// Queue all readers
	for _, reader := range s.activeReaders {
		readerChan <- reader
	}
	close(readerChan) // Signal no more work

	// Start worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go s.processWorker(batchCtx, readerChan, resultChan, cycleNum, &wg)
	}

	// Close the result channel when all workers are done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results from workers (channel-based, no locks!)
	totalOperations := 0
	var allLatencies []time.Duration
	timeoutCount := 0
	batchTimedOut := false

	for result := range resultChan {
		totalOperations += result.operationCount
		allLatencies = append(allLatencies, result.operationLatencies...)
		timeoutCount += result.timeoutCount
		s.stats.TotalOperations += int64(result.operationCount)

		// Check for batch timeout during the result collection
		if batchCtx.Err() != nil && !batchTimedOut {
			log.Printf("🚨 BATCH TIMEOUT: Cycle #%d exceeded %ds during result collection",
				cycleNum+1, int(BatchTimeoutSeconds))
			batchTimedOut = true
		}
	}

	return totalOperations, allLatencies, timeoutCount, batchTimedOut, numWorkers
}

// logBatchCompletion logs the completion of a batch with performance metrics.
func (s *UnifiedSimulation) logBatchCompletion(
	cycleNum int64,
	totalOperations int,
	batchDuration time.Duration,
	timeoutCount int,
	batchTimedOut bool,
	numWorkers int,
) {
	// Build timeout info string
	timeoutInfo := ""
	if timeoutCount > 0 {
		timeoutInfo = fmt.Sprintf(", %d timeouts", timeoutCount)
	}
	if batchTimedOut {
		timeoutInfo += " [BATCH TIMED OUT]"
	}

	// Log with the appropriate format based on whether operations occurred
	log.Printf("📊 Cycle #%d finished: %d ops in %v [%d workers]%s",
		cycleNum+1, totalOperations, batchDuration.Round(time.Millisecond), numWorkers, timeoutInfo)
}

// =================================================================
// AUTO-TUNING LOGIC - Immediate feedback from recent metrics
// =================================================================

// autoTuneFromRecentMetrics analyzes recent batch metrics and adjusts the active reader count.
func (s *UnifiedSimulation) autoTuneFromRecentMetrics() {
	// Calculate current performance from the latest batch
	avgLatency, throughput := s.calculateLatestBatchMetrics()
	timeoutRate := s.calculateTimeoutRateFromBatches()

	// Update stats
	s.stats.CurrentAvgLatencyMs = avgLatency.Milliseconds()
	s.stats.CurrentThroughputOpsPerSec = throughput
	s.stats.TimeoutRate = timeoutRate

	// Determine performance state
	performanceGood := s.isPerformanceGood(avgLatency, timeoutRate)
	performanceBad := s.isPerformanceBad(avgLatency, timeoutRate)

	// Calculate the adjustment
	adjustment, reason := s.calculateAdjustment(performanceGood, performanceBad)

	// Apply adjustment if needed
	if adjustment != 0 {
		s.applyAdjustment(adjustment, reason)
	}
}

// recordBatchMetrics adds batch metrics to the recent history.
func (s *UnifiedSimulation) recordBatchMetrics(metrics BatchMetrics) {
	s.recentBatches = append(s.recentBatches, metrics)

	// Maintain the sliding window
	if len(s.recentBatches) > s.maxBatchHistory {
		s.recentBatches = s.recentBatches[1:]
	}
}

// =================================================================
// METRICS CALCULATION - Direct from batch data
// =================================================================

// calculateLatestBatchMetrics calculates average latency and throughput from the most recent batch.
func (s *UnifiedSimulation) calculateLatestBatchMetrics() (avgLatency time.Duration, throughput float64) {
	if len(s.recentBatches) == 0 {
		return 0, 0
	}

	// Use only the latest batch
	latestBatch := s.recentBatches[len(s.recentBatches)-1]

	if latestBatch.OperationCount == 0 {
		return 0, 0
	}

	// Calculate the average latency for this batch
	avgLatency = s.calculateAverage(latestBatch.OperationLatencies)

	// Calculate throughput for this batch
	throughput = float64(latestBatch.OperationCount) / latestBatch.BatchDuration.Seconds()

	return avgLatency, throughput
}

// calculateTimeoutRateFromBatches calculates the timeout rate from recent batches.
func (s *UnifiedSimulation) calculateTimeoutRateFromBatches() float64 {
	totalTimeouts := 0
	totalAttempts := 0

	for _, batch := range s.recentBatches {
		totalTimeouts += batch.TimeoutCount
		totalAttempts += batch.OperationCount + batch.TimeoutCount // Finished operations + timeouts = total attempts
	}

	if totalAttempts == 0 {
		return 0
	}

	return float64(totalTimeouts) / float64(totalAttempts)
}

// calculateAverage calculates the average duration from a slice.
func (s *UnifiedSimulation) calculateAverage(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	var total time.Duration
	for _, d := range durations {
		total += d
	}

	return total / time.Duration(len(durations))
}

// =================================================================
// PERFORMANCE EVALUATION - Same logic as before, but simplified
// =================================================================

// isPerformanceGood determines if the current performance meets targets.
func (s *UnifiedSimulation) isPerformanceGood(avgLatency time.Duration, timeoutRate float64) bool {
	avgLatencyGood := avgLatency < time.Duration(TargetAvgLatencyMs)*time.Millisecond
	timeoutGood := timeoutRate < MaxTimeoutRate

	return avgLatencyGood && timeoutGood
}

// isPerformanceBad determines if the current performance is unacceptable.
func (s *UnifiedSimulation) isPerformanceBad(avgLatency time.Duration, timeoutRate float64) bool {
	// Use 1.5x the target as a "bad" threshold (simplified from MaxFactorForBadPerformance)
	badFactor := 1.5
	avgLatencyBad := avgLatency > time.Duration(float64(TargetAvgLatencyMs)*badFactor)*time.Millisecond
	timeoutBad := timeoutRate > MaxTimeoutRate*badFactor

	return avgLatencyBad || timeoutBad
}

// calculateAdjustment determines the adjustment needed based on performance.
func (s *UnifiedSimulation) calculateAdjustment(performanceGood, performanceBad bool) (adjustment int, reason string) {
	switch {
	case performanceGood && !performanceBad:
		s.consecutiveGoodCycles++
		s.consecutiveBadCycles = 0

		if s.consecutiveGoodCycles >= 2 {
			adjustment = ScaleUpIncrement
			reason = "good performance"
			s.consecutiveGoodCycles = 0
		}

	case performanceBad:
		s.consecutiveBadCycles++
		s.consecutiveGoodCycles = 0

		if s.consecutiveBadCycles >= 2 {
			adjustment = -ScaleDownIncrement
			reason = "performance degradation"
			s.consecutiveBadCycles = 0
		}

	default:
		s.consecutiveGoodCycles = 0
		s.consecutiveBadCycles = 0
		reason = "performance stable"
	}

	return adjustment, reason
}

// applyAdjustment adjusts the active reader count.
func (s *UnifiedSimulation) applyAdjustment(adjustment int, reason string) {
	currentCount := len(s.activeReaders)
	newCount := max(MinActiveReaders, min(currentCount+adjustment, MaxActiveReaders))

	if newCount != currentCount {
		s.adjustActiveReaderCount(newCount)

		if adjustment > 0 {
			s.stats.ScaleUpEvents++
			log.Printf("%s %s", AutoTune("📈"), AutoTune(fmt.Sprintf("AUTO-TUNE: Scaled UP to %d active readers (%s)", newCount, reason)))
		} else {
			s.stats.ScaleDownEvents++
			log.Printf("%s %s", AutoTune("📉"), AutoTune(fmt.Sprintf("AUTO-TUNE: Scaled DOWN to %d active readers (%s)", newCount, reason)))
		}

		s.stats.AutoTuneAdjustments++
		s.stats.LastAdjustmentReason = reason
	}
}

// rotateActiveReaders swaps ALL active readers to ensure fair participation.
func (s *UnifiedSimulation) rotateActiveReaders() {
	targetCount := len(s.activeReaders)
	if targetCount == 0 || len(s.inactiveReaders) == 0 {
		return
	}

	// Move all current active readers back to the inactive pool
	s.inactiveReaders = append(s.inactiveReaders, s.activeReaders...)
	s.activeReaders = s.activeReaders[:0] // Clear the active pool

	// Select new active readers using smart selection
	for i := 0; i < targetCount && len(s.inactiveReaders) > 0; i++ {
		selectedReader, randomIndex := s.selectReaderFromInactive()
		if selectedReader == nil {
			break // No more inactive readers available
		}

		// Move the reader from inactive to active
		s.inactiveReaders = append(s.inactiveReaders[:randomIndex], s.inactiveReaders[randomIndex+1:]...)
		s.activeReaders = append(s.activeReaders, selectedReader)
		selectedReader.Lifecycle = AtHome

		// Sync newly activated reader (10% chance)
		if rand.Float64() < ChanceSyncOnActivation { //nolint:gosec
			if err := s.syncSingleReader(selectedReader); err != nil {
				log.Printf("⚠️ Failed to sync rotated reader %s: %v", selectedReader.ID, err)
			}
		}
	}
}

// =================================================================
// HELPER METHODS - From original scheduler, simplified
// =================================================================

// selectReaderFromInactive selects a reader from the inactive pool using smart selection logic.
// Returns the selected reader and its index in the inactive readers slice.
func (s *UnifiedSimulation) selectReaderFromInactive() (*ReaderActor, int) {
	if len(s.inactiveReaders) == 0 {
		return nil, -1
	}

	preferReadersWithBooks := rand.Float32() < ChancePreferReadersWithBooks //nolint:gosec

	var selectedReader *ReaderActor
	var randomIndex int

	if preferReadersWithBooks {
		// Build the list of readers with borrowed books
		readersWithBooks := make([]int, 0, len(s.inactiveReaders))
		for idx, reader := range s.inactiveReaders {
			if len(reader.BorrowedBooks) > 0 {
				readersWithBooks = append(readersWithBooks, idx)
			}
		}

		if len(readersWithBooks) > 0 {
			selectedIdx := readersWithBooks[rand.Intn(len(readersWithBooks))] //nolint:gosec
			randomIndex = selectedIdx
			selectedReader = s.inactiveReaders[randomIndex]
		}
	}

	// Fallback to random selection if no preference match
	if selectedReader == nil {
		randomIndex = rand.Intn(len(s.inactiveReaders)) //nolint:gosec
		selectedReader = s.inactiveReaders[randomIndex]
	}

	return selectedReader, randomIndex
}

// adjustActiveReaderCount changes the number of active readers.
func (s *UnifiedSimulation) adjustActiveReaderCount(targetCount int) {
	currentCount := len(s.activeReaders)
	targetCount = max(MinActiveReaders, min(targetCount, MaxActiveReaders))

	if targetCount > currentCount {
		// Need more active readers
		needed := targetCount - currentCount
		available := len(s.inactiveReaders)
		toActivate := min(needed, available)

		for i := 0; i < toActivate && len(s.inactiveReaders) > 0; i++ {
			// Use smart reader selection
			selectedReader, randomIndex := s.selectReaderFromInactive()
			if selectedReader == nil {
				break // No more inactive readers available
			}

			// Move reader to active pool
			s.inactiveReaders = append(s.inactiveReaders[:randomIndex], s.inactiveReaders[randomIndex+1:]...)
			s.activeReaders = append(s.activeReaders, selectedReader)
			selectedReader.Lifecycle = AtHome

			// Sync newly activated reader
			if rand.Float64() < ChanceSyncOnActivation { //nolint:gosec
				if err := s.syncSingleReader(selectedReader); err != nil {
					log.Printf("⚠️ Failed to sync newly activated reader %s: %v", selectedReader.ID, err)
				}
			}
		}

		if toActivate > 0 {
			log.Printf("%s %s", AutoTune("📈"), AutoTune(fmt.Sprintf("Activated %d readers (%d -> %d active)", toActivate, currentCount, len(s.activeReaders))))
		}

	} else if targetCount < currentCount {
		// Need fewer active readers
		excess := currentCount - targetCount

		for i := 0; i < excess; i++ {
			// Move the oldest reader from active to inactive (FIFO)
			reader := s.activeReaders[0]
			s.activeReaders = s.activeReaders[1:]
			s.inactiveReaders = append(s.inactiveReaders, reader)
			reader.Lifecycle = AtHome
		}

		log.Printf("📉 Deactivated %d readers (%d -> %d active)", excess, currentCount, len(s.activeReaders))
	}

	s.stats.CurrentActiveReaders = len(s.activeReaders)
}

// processLibrarians handles librarian work.
func (s *UnifiedSimulation) processLibrarians() {
	for _, librarian := range s.librarians {
		if s.ctx.Err() != nil {
			return
		}

		operationCount, err := librarian.Work(s.ctx, s.handlers, s.state)
		if err != nil {
			log.Printf("⚠️  Librarian %s work failed: %v", librarian.ID, err)
		}
		s.stats.TotalOperations += int64(operationCount)
	}
}

// adjustPopulationIfNeeded manages reader registration and cancellation.
func (s *UnifiedSimulation) adjustPopulationIfNeeded() {
	totalReaders := len(s.activeReaders) + len(s.inactiveReaders)

	if totalReaders < MinReaders {
		// Need more readers
		newReaders := min(10, MinReaders-totalReaders)
		for i := 0; i < newReaders; i++ {
			reader := NewReaderActor(CasualReader)
			reader.Lifecycle = Registered

			if err := s.handlers.ExecuteRegisterReader(s.ctx, reader.ID); err != nil {
				log.Printf("⚠️  Failed to register reader %s: %v", reader.ID, err)
				continue
			}

			s.inactiveReaders = append(s.inactiveReaders, reader)
		}
		log.Printf("📈 Registered %d new readers (total: %d)", newReaders, totalReaders+newReaders)

	} else if totalReaders > MaxReaders {
		// Too many readers
		readersToCancel := min(5, totalReaders-MaxReaders)
		s.cancelExcessReaders(readersToCancel)
		log.Printf("📉 Canceled %d readers (total: %d)", readersToCancel, totalReaders-readersToCancel)
	}
}

// handleReaderCancellation processes a reader cancellation.
func (s *UnifiedSimulation) handleReaderCancellation(reader *ReaderActor) {
	// Only cancel if the reader has no borrowed books
	borrowedBooks := s.state.GetReaderBorrowedBooks(reader.ID)
	if len(borrowedBooks) > 0 {
		return
	}

	if err := s.handlers.ExecuteCancelReader(s.ctx, reader.ID); err != nil {
		log.Printf("%s Failed to cancel reader contract %s: %s", BusinessError("⚠️"), reader.ID, BusinessError(err.Error()))
		return
	}

	reader.Lifecycle = Canceled
	s.state.CancelReader(reader.ID)

	// Remove from active pool in next rotation
}

// cancelExcessReaders removes readers when above the maximum.
func (s *UnifiedSimulation) cancelExcessReaders(count int) {
	canceled := 0

	// Cancel from inactive readers first
	for i := len(s.inactiveReaders) - 1; i >= 0 && canceled < count; i-- {
		reader := s.inactiveReaders[i]

		borrowedBooks := s.state.GetReaderBorrowedBooks(reader.ID)
		if len(borrowedBooks) == 0 {
			if err := s.handlers.ExecuteCancelReader(s.ctx, reader.ID); err != nil {
				log.Printf("%s Failed to cancel reader contract %s: %s", BusinessError("⚠️"), reader.ID, BusinessError(err.Error()))
				continue
			}

			s.inactiveReaders = append(s.inactiveReaders[:i], s.inactiveReaders[i+1:]...)
			s.state.CancelReader(reader.ID)
			reader.Lifecycle = Canceled
			canceled++
		}
	}
}

// syncSingleReader synchronizes borrowed books for a reader.
func (s *UnifiedSimulation) syncSingleReader(actor *ReaderActor) error {
	readerBooks, err := s.handlers.QueryBooksLentByReader(s.ctx, actor.ID)
	if err != nil {
		return fmt.Errorf("failed to query books for reader %s: %w", actor.ID, err)
	}

	borrowedBooks := make([]uuid.UUID, 0, len(readerBooks.Books))
	for _, book := range readerBooks.Books {
		bookID, err := uuid.Parse(book.BookID)
		if err != nil {
			log.Printf("⚠️ Invalid BookID '%s' for reader %s", book.BookID, actor.ID)
			continue
		}
		borrowedBooks = append(borrowedBooks, bookID)
	}

	actor.BorrowedBooks = borrowedBooks
	return nil
}

// synchronizeActorBorrowedBooksFromLoadedState populates ALL actor BorrowedBooks from the loaded state.
func (s *UnifiedSimulation) synchronizeActorBorrowedBooksFromLoadedState() {
	stats := s.state.GetStats()
	totalLentBooks := stats.BooksLentOut

	readerBooksMap := s.state.GetReaderBooksMap()

	totalActorsPopulated := 0
	totalBooksAssigned := 0

	allReaders := make([]*ReaderActor, 0, len(s.activeReaders)+len(s.inactiveReaders))
	allReaders = append(allReaders, s.activeReaders...)
	allReaders = append(allReaders, s.inactiveReaders...)

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

// processWorker processes readers from a channel using the worker pool pattern.
func (s *UnifiedSimulation) processWorker(
	ctx context.Context,
	readers <-chan *ReaderActor, // receive-only channel
	results chan<- batchResult, // send-only channel
	cycleNum int64,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	result := batchResult{
		operationLatencies: make([]time.Duration, 0),
	}

	// Pull from the channel until empty or canceled
	for reader := range readers {
		// Check context BEFORE processing each reader
		if ctx.Err() != nil {
			result.timeoutCount++
			result.readersProcessed++
			continue
		}

		if reader.ShouldVisitLibrary() {
			operationStart := time.Now()

			operationCount, err := reader.VisitLibrary(ctx, s.handlers)
			operationDuration := time.Since(operationStart)

			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
					result.timeoutCount++
					log.Printf("⏱️ Reader %s operation timed out in cycle #%d (%v)", reader.ID, cycleNum+1, operationDuration)
				} else {
					log.Printf("⚠️  Reader %s library visit failed in cycle #%d: %v", reader.ID, cycleNum+1, err)
				}
			} else {
				// Record successful operations
				for i := 0; i < operationCount; i++ {
					result.operationLatencies = append(result.operationLatencies, operationDuration/time.Duration(operationCount))
				}
				result.operationCount += operationCount
			}
		}

		// Handle contract cancellation (only if no context timeout)
		if ctx.Err() == nil && reader.ShouldCancelContract() {
			s.handleReaderCancellation(reader)
		}

		result.readersProcessed++

		// Check context AFTER processing too (early exit on timeout)
		if ctx.Err() != nil {
			break // Stop processing on timeout
		}
	}

	// Send accumulated result (non-blocking)
	select {
	case results <- result:
	case <-ctx.Done():
		// Context canceled, don't block on sending to the channel
	}
}

func (s *UnifiedSimulation) logPerformance() {
	// Get book statistics for comprehensive logging
	stateStats := s.state.GetStats()

	log.Printf("%s %s",
		Performance("🎯"),
		Performance(fmt.Sprintf("Performance: avg=%dms/op, %.1f ops/sec, timeouts=%.1f%%, active=%d readers | Books: %d total, %d lent out",
			s.stats.CurrentAvgLatencyMs, s.stats.CurrentThroughputOpsPerSec, s.stats.TimeoutRate*100,
			len(s.activeReaders), stateStats.TotalBooks, stateStats.BooksLentOut)))
}
