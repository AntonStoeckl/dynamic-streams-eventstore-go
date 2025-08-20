package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"sort"
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

// BatchMetrics contains performance data from a single batch
type BatchMetrics struct {
	OperationCount     int
	OperationLatencies []time.Duration
	TimeoutCount       int
	BatchDuration      time.Duration
	Timestamp          time.Time
}

// UnifiedStats tracks overall simulation performance
type UnifiedStats struct {
	TotalCycles          int64
	TotalOperations      int64
	AutoTuneAdjustments  int64
	ScaleUpEvents        int64
	ScaleDownEvents      int64
	CurrentActiveReaders int
	CurrentP50Ms         int64
	CurrentP99Ms         int64
	TimeoutRate          float64
	LastAdjustmentReason string
}

// NewUnifiedSimulation creates a new unified simulation
func NewUnifiedSimulation(
	ctx context.Context,
	eventStore *postgresengine.EventStore,
	state *SimulationState,
	handlers *HandlerBundle,
) (*UnifiedSimulation, error) {
	simCtx, cancel := context.WithCancel(ctx)

	sim := &UnifiedSimulation{
		ctx:        simCtx,
		cancel:     cancel,
		eventStore: eventStore,
		state:      state,
		handlers:   handlers,
		stopChan:   make(chan struct{}),

		activeReaders:   make([]*ReaderActor, 0, MaxActiveReaders),
		inactiveReaders: make([]*ReaderActor, 0, MaxReaders),
		librarians:      make([]*LibrarianActor, 0, LibrarianCount),

		recentBatches:            make([]BatchMetrics, 0, 10),
		maxBatchHistory:          10, // 1 second of history
		targetActiveCount:        InitialActiveReaders,
		lastPopulationAdjustment: time.Now(),
	}

	// Initialize actor pools
	if err := sim.initializeActorPools(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize actor pools: %w", err)
	}

	return sim, nil
}

// initializeActorPools creates the initial reader and librarian populations
//
//nolint:funlen
func (s *UnifiedSimulation) initializeActorPools() error {
	fmt.Printf("%s %s\n", StatusIcon("working"), Info("Initializing actor pools..."))

	// Load all state from the database first
	log.Printf("📚 Loading complete simulation state from EventStore...")
	if err := s.state.RefreshFromEventStore(s.ctx, s.handlers); err != nil {
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

// Start begins the unified simulation loop
func (s *UnifiedSimulation) Start() error {
	log.Printf("%s %s", Success("🚀"), Success("Starting unified simulation..."))
	log.Printf("🎯 Auto-tune targets: P50<%dms, P99<%dms, timeouts<%.1f%%",
		TargetP50LatencyMs, TargetP99LatencyMs, MaxTimeoutRate*100)

	// Run the main simulation loop
	return s.runMainLoop()
}

// Stop gracefully shuts down the simulation
func (s *UnifiedSimulation) Stop() error {
	log.Printf("🛑 Stopping unified simulation...")

	close(s.stopChan)
	s.cancel()

	log.Printf("✅ Unified simulation stopped")
	return nil
}

// =================================================================
// MAIN SIMULATION LOOP - The heart of the simplified architecture
// =================================================================

// runMainLoop is the single, sequential loop that handles everything
func (s *UnifiedSimulation) runMainLoop() error {
	cycleNum := int64(0)
	log.Printf("%s %s", AutoTune("🔄"), AutoTune("Main simulation loop started"))

	for {
		select {
		case <-s.stopChan:
			return nil
		case <-s.ctx.Done():
			return nil
		default:
			cycleStart := time.Now()

			// EVERY batch: Core processing
			s.rotateActiveReaders()
			batchMetrics := s.processBatch(cycleNum)
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

// processBatch processes all active readers and returns metrics
func (s *UnifiedSimulation) processBatch(cycleNum int64) BatchMetrics {
	batchStart := time.Now()

	if len(s.activeReaders) == 0 {
		return BatchMetrics{Timestamp: batchStart}
	}

	// Create batch context with timeout
	batchCtx, batchCancel := context.WithTimeout(s.ctx, BatchTimeoutSeconds*time.Second)
	defer batchCancel()

	// Get book stats for logging
	log.Printf("📊 Cycle #%d started", cycleNum+1)

	totalOperations := 0
	var allLatencies []time.Duration
	timeoutCount := 0

	// Track batch timeout state
	batchTimedOut := false
	readersProcessed := 0

	// Process each reader
	for _, reader := range s.activeReaders {
		// Check for batch timeout more frequently
		if batchCtx.Err() != nil {
			if !batchTimedOut {
				log.Printf("🚨 BATCH TIMEOUT: Cycle #%d exceeded %ds after processing %d/%d readers",
					cycleNum+1, int(BatchTimeoutSeconds), readersProcessed, len(s.activeReaders))
				batchTimedOut = true
			}
			timeoutCount++
			continue
		}

		if reader.ShouldVisitLibrary() {
			operationStart := time.Now()

			operationCount, err := reader.VisitLibrary(batchCtx, s.handlers)
			operationDuration := time.Since(operationStart)

			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
					timeoutCount++
					if batchTimedOut {
						log.Printf("⏱️ Reader %s canceled due to batch timeout", reader.ID)
					} else {
						log.Printf("⏱️ Reader %s operation timed out (%v)", reader.ID, operationDuration)
					}
				} else {
					log.Printf("⚠️  Reader %s library visit failed: %v", reader.ID, err)
				}
			} else {
				// Record successful operations
				for i := 0; i < operationCount; i++ {
					allLatencies = append(allLatencies, operationDuration/time.Duration(operationCount))
				}
				totalOperations += operationCount
				s.stats.TotalOperations += int64(operationCount)
			}
		}

		// Handle contract cancellation (only if no batch timeout)
		if !batchTimedOut && reader.ShouldCancelContract() {
			s.handleReaderCancellation(reader)
		}

		readersProcessed++
	}

	batchDuration := time.Since(batchStart)

	// Log batch completion with timeout info
	timeoutInfo := ""
	if timeoutCount > 0 {
		timeoutInfo = fmt.Sprintf(", %d timeouts", timeoutCount)
	}
	if batchTimedOut {
		timeoutInfo += " [BATCH TIMED OUT]"
	}

	if totalOperations > 0 {
		avgLatency := s.calculateAverage(allLatencies)
		throughput := float64(totalOperations) / batchDuration.Seconds()

		log.Printf("📊 Cycle #%d finished: %d ops in %v (avg: %v/op, %.1f ops/sec)%s",
			cycleNum+1, totalOperations, batchDuration.Round(time.Millisecond),
			avgLatency.Round(time.Millisecond), throughput, timeoutInfo)
	} else {
		log.Printf("📊 Cycle #%d finished: %d operations in %v%s",
			cycleNum+1, totalOperations, batchDuration.Round(time.Millisecond), timeoutInfo)
	}

	return BatchMetrics{
		OperationCount:     totalOperations,
		OperationLatencies: allLatencies,
		TimeoutCount:       timeoutCount,
		BatchDuration:      batchDuration,
		Timestamp:          batchStart,
	}
}

// =================================================================
// AUTO-TUNING LOGIC - Immediate feedback from recent metrics
// =================================================================

// autoTuneFromRecentMetrics analyzes recent batch metrics and adjusts active reader count
func (s *UnifiedSimulation) autoTuneFromRecentMetrics() {
	// Calculate current performance from recent batches
	p50, p99 := s.calculatePercentilesFromBatches()
	timeoutRate := s.calculateTimeoutRateFromBatches()

	// Update stats
	s.stats.CurrentP50Ms = p50.Milliseconds()
	s.stats.CurrentP99Ms = p99.Milliseconds()
	s.stats.TimeoutRate = timeoutRate

	// Determine performance state
	performanceGood := s.isPerformanceGood(p50, p99, timeoutRate)
	performanceBad := s.isPerformanceBad(p50, p99, timeoutRate)

	// Calculate adjustment
	adjustment, reason := s.calculateAdjustment(performanceGood, performanceBad)

	// Apply adjustment if needed
	if adjustment != 0 {
		s.applyAdjustment(adjustment, reason)
	}
}

// recordBatchMetrics adds batch metrics to recent history
func (s *UnifiedSimulation) recordBatchMetrics(metrics BatchMetrics) {
	s.recentBatches = append(s.recentBatches, metrics)

	// Maintain sliding window
	if len(s.recentBatches) > s.maxBatchHistory {
		s.recentBatches = s.recentBatches[1:]
	}
}

// =================================================================
// METRICS CALCULATION - Direct from batch data
// =================================================================

// calculatePercentilesFromBatches calculates P50 and P99 from recent batch metrics
func (s *UnifiedSimulation) calculatePercentilesFromBatches() (p50, p99 time.Duration) {
	var allLatencies []time.Duration

	for _, batch := range s.recentBatches {
		allLatencies = append(allLatencies, batch.OperationLatencies...)
	}

	if len(allLatencies) == 0 {
		return 0, 0
	}

	// Sort latencies
	sort.Slice(allLatencies, func(i, j int) bool {
		return allLatencies[i] < allLatencies[j]
	})

	// Calculate percentiles
	p50Index := int(float64(len(allLatencies)) * 0.5)
	p99Index := int(float64(len(allLatencies)) * 0.99)

	if p50Index >= len(allLatencies) {
		p50Index = len(allLatencies) - 1
	}
	if p99Index >= len(allLatencies) {
		p99Index = len(allLatencies) - 1
	}

	return allLatencies[p50Index], allLatencies[p99Index]
}

// calculateTimeoutRateFromBatches calculates timeout rate from recent batches
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

// calculateAverage calculates average duration from slice
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

// isPerformanceGood determines if current performance meets targets
func (s *UnifiedSimulation) isPerformanceGood(p50, p99 time.Duration, timeoutRate float64) bool {
	p50Good := p50 < time.Duration(TargetP50LatencyMs)*time.Millisecond
	p99Good := p99 < time.Duration(TargetP99LatencyMs)*time.Millisecond
	timeoutGood := timeoutRate < MaxTimeoutRate

	return p50Good && p99Good && timeoutGood
}

// =================================================================
// PERFORMANCE EVALUATION - Same logic as before, but simplified
// =================================================================

// isPerformanceBad determines if current performance is unacceptable
func (s *UnifiedSimulation) isPerformanceBad(p50, p99 time.Duration, timeoutRate float64) bool {
	badFactor := MaxFactorForBadPerformance
	p99Bad := p99 > time.Duration(TargetP99LatencyMs*badFactor)*time.Millisecond
	p50Bad := p50 > time.Duration(TargetP50LatencyMs*badFactor)*time.Millisecond
	timeoutBad := timeoutRate > MaxTimeoutRate*badFactor

	return p99Bad || p50Bad || timeoutBad
}

// calculateAdjustment determines adjustment needed based on performance
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

// applyAdjustment adjusts the active reader count
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

// rotateActiveReaders swaps ALL active readers to ensure fair participation
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
		preferReadersWithBooks := rand.Float32() < ChancePreferReadersWithBooks //nolint:gosec

		var selectedReader *ReaderActor
		var randomIndex int

		if preferReadersWithBooks {
			// Build a list of inactive readers with books
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

		// Fallback to random selection
		if selectedReader == nil {
			randomIndex = rand.Intn(len(s.inactiveReaders)) //nolint:gosec
			selectedReader = s.inactiveReaders[randomIndex]
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

// adjustActiveReaderCount changes the number of active readers
func (s *UnifiedSimulation) adjustActiveReaderCount(targetCount int) {
	currentCount := len(s.activeReaders)
	targetCount = max(MinActiveReaders, min(targetCount, MaxActiveReaders))

	if targetCount > currentCount {
		// Need more active readers
		needed := targetCount - currentCount
		available := len(s.inactiveReaders)
		toActivate := min(needed, available)

		for i := 0; i < toActivate && len(s.inactiveReaders) > 0; i++ {
			// Smart reader selection
			preferReadersWithBooks := rand.Float32() < ChancePreferReadersWithBooks //nolint:gosec

			var selectedReader *ReaderActor
			var randomIndex int

			if preferReadersWithBooks {
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

			if selectedReader == nil {
				randomIndex = rand.Intn(len(s.inactiveReaders)) //nolint:gosec
				selectedReader = s.inactiveReaders[randomIndex]
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

// processLibrarians handles librarian work
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

// adjustPopulationIfNeeded manages reader registration/cancellation
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

// handleReaderCancellation processes a reader cancellation
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

// cancelExcessReaders removes readers when above maximum
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

// syncSingleReader synchronizes borrowed books for a reader
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

// synchronizeActorBorrowedBooksFromLoadedState populates ALL actor BorrowedBooks from loaded state
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

func (s *UnifiedSimulation) logPerformance() {
	// Get book statistics for comprehensive logging
	stateStats := s.state.GetStats()

	log.Printf("%s %s",
		Performance("🎯"),
		Performance(fmt.Sprintf("Performance: P50=%dms, P99=%dms, timeouts=%.1f%%, active=%d readers | Books: %d total, %d lent out",
			s.stats.CurrentP50Ms, s.stats.CurrentP99Ms, s.stats.TimeoutRate*100,
			len(s.activeReaders), stateStats.TotalBooks, stateStats.BooksLentOut)))
}
