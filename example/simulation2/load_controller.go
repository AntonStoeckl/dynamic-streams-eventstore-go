package main

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"
)

// =================================================================
// LOAD CONTROLLER - Auto-tuning system for optimal performance
// =================================================================

// LoadController automatically adjusts the number of active actors based on system performance.
// This is the "intelligence" that discovers the system's natural capacity.
type LoadController struct {
	// Context and control
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	stopChan chan struct{}

	// Dependencies
	scheduler *ActorScheduler
	state     *SimulationState

	// Performance metrics
	mu                sync.RWMutex
	latencyHistory    []time.Duration // Recent operation latencies
	timeoutHistory    []bool          // Recent timeout occurrences
	throughputHistory []float64       // Recent operations per second

	// Current performance state
	currentP50        time.Duration
	currentP99        time.Duration
	currentTimeout    float64 // Percentage of operations that timed out
	currentThroughput float64 // Operations per second

	// Auto-tuning state
	lastAdjustment             time.Time
	consecutiveGoodPerformance int
	consecutiveBadPerformance  int

	// Throughput calculation
	operationCount     int64     // Total operations since last throughput calculation
	lastThroughputCalc time.Time // When we last calculated throughput

	// Statistics
	stats LoadControllerStats
}

// LoadControllerStats tracks auto-tuning performance and decisions.
type LoadControllerStats struct {
	TotalAdjustments     int64
	ScaleUpEvents        int64
	ScaleDownEvents      int64
	CurrentActiveReaders int
	TargetLatencyMs      int
	CurrentP50Ms         int64
	CurrentP99Ms         int64
	TimeoutRate          float64
	Throughput           float64
	LastAdjustmentReason string
}

// NewLoadController creates a new auto-tuning load controller.
func NewLoadController(ctx context.Context, scheduler *ActorScheduler, state *SimulationState) *LoadController {
	controllerCtx, cancel := context.WithCancel(ctx)

	return &LoadController{
		ctx:       controllerCtx,
		cancel:    cancel,
		stopChan:  make(chan struct{}),
		scheduler: scheduler,
		state:     state,

		latencyHistory:    make([]time.Duration, 0, MetricsWindowSize),
		timeoutHistory:    make([]bool, 0, MetricsWindowSize),
		throughputHistory: make([]float64, 0, MetricsWindowSize),

		lastAdjustment:     time.Now(),
		lastThroughputCalc: time.Now(),
	}
}

// Start begins the auto-tuning process.
func (lc *LoadController) Start() error {
	log.Printf("%s %s", AutoTune("🧠"), AutoTune("Starting load controller auto-tuning..."))
	log.Printf("🎯 Targets: P50<%dms, P99<%dms, timeouts<%.1f%%",
		TargetP50LatencyMs, TargetP99LatencyMs, MaxTimeoutRate*100)

	lc.wg.Add(1)
	go lc.tuningLoop()

	return nil
}

// Stop gracefully shuts down the load controller.
func (lc *LoadController) Stop() error {
	log.Printf("🛑 Stopping load controller...")

	close(lc.stopChan)
	lc.cancel()

	// Wait for the tuning loop to finish
	done := make(chan struct{})
	go func() {
		lc.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Printf("✅ Load controller stopped gracefully")
	case <-time.After(2 * time.Second):
		log.Printf("⚠️  Load controller stop timeout")
	}

	return nil
}

// =================================================================
// METRICS COLLECTION - Performance monitoring
// =================================================================

// RecordLatency records operation latency for performance analysis.
func (lc *LoadController) RecordLatency(duration time.Duration) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	// Add to history, maintain window size
	lc.latencyHistory = append(lc.latencyHistory, duration)
	if len(lc.latencyHistory) > MetricsWindowSize {
		lc.latencyHistory = lc.latencyHistory[1:]
	}

	// Count this operation for throughput calculation
	lc.operationCount++
	lc.calculateThroughputIfNeeded()

	// Update current metrics
	lc.updateCurrentMetrics()
}

// RecordTimeout records whether an operation timed out.
func (lc *LoadController) RecordTimeout(timedOut bool) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	// Add to history, maintain window size
	lc.timeoutHistory = append(lc.timeoutHistory, timedOut)
	if len(lc.timeoutHistory) > MetricsWindowSize {
		lc.timeoutHistory = lc.timeoutHistory[1:]
	}

	// Update current metrics
	lc.updateCurrentMetrics()
}

// RecordThroughput records current throughput (operations per second).
func (lc *LoadController) RecordThroughput(opsPerSecond float64) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	// Add to history, maintain window size
	lc.throughputHistory = append(lc.throughputHistory, opsPerSecond)
	if len(lc.throughputHistory) > MetricsWindowSize {
		lc.throughputHistory = lc.throughputHistory[1:]
	}

	lc.currentThroughput = opsPerSecond
}

// calculateThroughputIfNeeded calculates and records throughput periodically.
func (lc *LoadController) calculateThroughputIfNeeded() {
	now := time.Now()
	timeSinceLastCalc := now.Sub(lc.lastThroughputCalc)

	// Calculate throughput every 2 seconds
	if timeSinceLastCalc >= 2*time.Second {
		if timeSinceLastCalc > 0 {
			// Calculate operations per second
			opsPerSecond := float64(lc.operationCount) / timeSinceLastCalc.Seconds()

			// Record the throughput
			lc.throughputHistory = append(lc.throughputHistory, opsPerSecond)
			if len(lc.throughputHistory) > MetricsWindowSize {
				lc.throughputHistory = lc.throughputHistory[1:]
			}

			lc.currentThroughput = opsPerSecond

			// Reset counters
			lc.operationCount = 0
			lc.lastThroughputCalc = now
		}
	}
}

// updateCurrentMetrics calculates current performance metrics from history.
func (lc *LoadController) updateCurrentMetrics() {
	// Calculate latency percentiles
	if len(lc.latencyHistory) > 0 {
		lc.currentP50 = lc.calculatePercentile(lc.latencyHistory, 0.50)
		lc.currentP99 = lc.calculatePercentile(lc.latencyHistory, 0.99)
	}

	// Calculate timeout rate
	if len(lc.timeoutHistory) > 0 {
		timeouts := 0
		for _, timedOut := range lc.timeoutHistory {
			if timedOut {
				timeouts++
			}
		}
		lc.currentTimeout = float64(timeouts) / float64(len(lc.timeoutHistory))
	}
}

// calculatePercentile calculates the specified percentile from latency data.
func (lc *LoadController) calculatePercentile(latencies []time.Duration, percentile float64) time.Duration {
	if len(latencies) == 0 {
		return 0
	}

	// Create a sorted copy
	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	// Calculate the index for the percentile
	index := int(float64(len(sorted)-1) * percentile)
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}

	return sorted[index]
}

// =================================================================
// AUTO-TUNING LOGIC - The intelligence of the system
// =================================================================

// tuningLoop is the main auto-tuning control loop.
func (lc *LoadController) tuningLoop() {
	defer lc.wg.Done()

	ticker := time.NewTicker(time.Duration(TuningIntervalSeconds) * time.Second)
	defer ticker.Stop()

	log.Printf("%s %s", AutoTune("🔄"), AutoTune(fmt.Sprintf("Auto-tuning loop started (adjusts every %ds)", TuningIntervalSeconds)))

	for {
		select {
		case <-lc.stopChan:
			return
		case <-lc.ctx.Done():
			return
		case <-ticker.C:
			lc.evaluateAndAdjust()
		}
	}
}

// evaluateAndAdjust analyzes current performance and adjusts active reader count.
func (lc *LoadController) evaluateAndAdjust() {
	// CRITICAL: Get scheduler stats BEFORE acquiring our lock to prevent deadlock
	// Deadlock scenario: LoadController.Lock() → ActorScheduler.RLock() vs
	//                    processReaderBatch → LoadController.Lock()
	schedulerStats := lc.scheduler.GetStats()

	lc.mu.Lock()
	defer lc.mu.Unlock()

	currentActiveReaders := schedulerStats.ActiveReaderCount

	// Determine if performance is good or bad
	performanceGood := lc.isPerformanceGood()
	performanceBad := lc.isPerformanceBad()

	var adjustment int
	var reason string

	adjustment, reason = lc.calculateAdjustment(performanceGood, performanceBad)

	// Apply adjustment if needed
	lc.applyAdjustment(adjustment, reason, currentActiveReaders)

	// Update statistics
	lc.updateStats(currentActiveReaders)

	// Validate metrics sanity and warn about anomalies
	if lc.currentP99 > 0 && lc.currentP50 > 0 && lc.currentP99 > AnomalyDetectionMultiplier*lc.currentP50 {
		log.Printf("⚠️  Metrics anomaly detected: P99=%v is >%.1fx P50=%v (likely outlier)",
			lc.currentP99, AnomalyDetectionMultiplier, lc.currentP50)
	}

	// Log performance every evaluation (every 5 seconds)
	// Get book statistics for comprehensive system overview
	stateStats := lc.state.GetStats()

	log.Printf("%s %s",
		Performance("🎯"),
		Performance(fmt.Sprintf("Performance: P50=%dms, P99=%dms, timeouts=%.1f%%, throughput=%.1f ops/s, active=%d readers | Books: %d total, %d lent out",
			lc.stats.CurrentP50Ms, lc.stats.CurrentP99Ms, lc.stats.TimeoutRate*100,
			lc.stats.Throughput, currentActiveReaders, stateStats.TotalBooks, stateStats.BooksLentOut)))
}

// isPerformanceGood determines if the current performance meets targets.
func (lc *LoadController) isPerformanceGood() bool {
	// Need sufficient data
	if len(lc.latencyHistory) < 10 {
		return false
	}

	// All metrics must be within targets
	p50Good := lc.currentP50 < time.Duration(TargetP50LatencyMs)*time.Millisecond
	p99Good := lc.currentP99 < time.Duration(TargetP99LatencyMs)*time.Millisecond
	timeoutGood := lc.currentTimeout < MaxTimeoutRate

	return p50Good && p99Good && timeoutGood
}

// isPerformanceBad determines if current performance is unacceptable.
func (lc *LoadController) isPerformanceBad() bool {
	// Need sufficient data
	if len(lc.latencyHistory) < 5 {
		return false
	}

	// Any critical metric failure indicates bad performance
	p99Bad := lc.currentP99 > time.Duration(TargetP99LatencyMs*3)*time.Millisecond // 3x target (750ms) to tolerate librarian queries
	timeoutBad := lc.currentTimeout > MaxTimeoutRate*2                             // 2x target

	return p99Bad || timeoutBad
}

// updateStats updates the controller statistics.
func (lc *LoadController) updateStats(currentActiveReaders int) {
	lc.stats.CurrentActiveReaders = currentActiveReaders
	lc.stats.TargetLatencyMs = TargetP50LatencyMs
	lc.stats.CurrentP50Ms = lc.currentP50.Milliseconds()
	lc.stats.CurrentP99Ms = lc.currentP99.Milliseconds()
	lc.stats.TimeoutRate = lc.currentTimeout
	lc.stats.Throughput = lc.currentThroughput
}

// =================================================================
// PUBLIC API - For integration with main simulation
// =================================================================

// GetStats returns current load controller statistics.
func (lc *LoadController) GetStats() LoadControllerStats {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	return lc.stats
}

// GetCurrentMetrics returns current performance metrics.
func (lc *LoadController) GetCurrentMetrics() (p50, p99 time.Duration, timeoutRate float64) {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	return lc.currentP50, lc.currentP99, lc.currentTimeout
}

// IsSystemHealthy returns true if the system is performing well.
func (lc *LoadController) IsSystemHealthy() bool {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	return lc.isPerformanceGood() && !lc.isPerformanceBad()
}

// GetRecommendedActiveReaders returns the controller's recommended active reader count.
func (lc *LoadController) GetRecommendedActiveReaders() int {
	// CRITICAL: Get scheduler stats BEFORE acquiring our lock to prevent deadlock
	// Same deadlock scenario as evaluateAndAdjust()
	schedulerStats := lc.scheduler.GetStats()

	// Note: We don't actually need our lock here since we're just returning
	// the scheduler's current count, but kept pattern for consistency
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	// Return the current scheduler target (this may have been adjusted)
	return schedulerStats.ActiveReaderCount
}

// calculateAdjustment determines the adjustment needed based on performance.
func (lc *LoadController) calculateAdjustment(performanceGood, performanceBad bool) (adjustment int, reason string) {
	switch {
	case performanceGood && !performanceBad:
		// Performance is good - consider scaling up
		lc.consecutiveGoodPerformance++
		lc.consecutiveBadPerformance = 0

		if lc.consecutiveGoodPerformance >= 2 { // Need consistent good performance
			adjustment = ScaleUpIncrement
			reason = "consistent good performance"
			lc.consecutiveGoodPerformance = 0 // Reset after scaling
		}

	case performanceBad:
		// Performance is bad - consider scaling down
		lc.consecutiveBadPerformance++
		lc.consecutiveGoodPerformance = 0

		if lc.consecutiveBadPerformance >= 2 { // Need consistent bad performance to avoid single spike reactions
			adjustment = -ScaleDownIncrement
			reason = "consistent performance degradation detected"
			lc.consecutiveBadPerformance = 0 // Reset after scaling
		}

	default:
		// Performance is neutral - no change
		lc.consecutiveGoodPerformance = 0
		lc.consecutiveBadPerformance = 0
		reason = "performance stable"
	}

	return adjustment, reason
}

// applyAdjustment applies the calculated adjustment to the active reader count.
func (lc *LoadController) applyAdjustment(adjustment int, reason string, currentActiveReaders int) {
	if adjustment == 0 {
		return
	}

	newActiveCount := max(MinActiveReaders, min(currentActiveReaders+adjustment, MaxActiveReaders))

	if newActiveCount != currentActiveReaders {
		lc.scheduler.AdjustActiveReaderCount(newActiveCount)

		if adjustment > 0 {
			lc.stats.ScaleUpEvents++
			log.Printf("%s %s", AutoTune("📈"), AutoTune(fmt.Sprintf("AUTO-TUNE: Scaled UP to %d active readers (%s)", newActiveCount, reason)))
		} else {
			lc.stats.ScaleDownEvents++
			log.Printf("%s %s", AutoTune("📉"), AutoTune(fmt.Sprintf("AUTO-TUNE: Scaled DOWN to %d active readers (%s)", newActiveCount, reason)))
		}

		lc.stats.TotalAdjustments++
		lc.stats.LastAdjustmentReason = reason
		lc.lastAdjustment = time.Now()
	}
}
