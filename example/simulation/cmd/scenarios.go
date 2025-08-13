package main

import (
	"math/rand"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ScenarioType represents different types of scenarios the simulation can execute.
type ScenarioType string

const (
	ScenarioAddBook        ScenarioType = "add_book"
	ScenarioRemoveBook     ScenarioType = "remove_book"
	ScenarioRegisterReader ScenarioType = "register_reader"
	ScenarioCancelReader   ScenarioType = "cancel_reader"
	ScenarioLendBook       ScenarioType = "lend_book"
	ScenarioReturnBook     ScenarioType = "return_book"
	ScenarioQueryBooks     ScenarioType = "query_books_lent_by_reader" // 5% of returns
)

// Scenario represents a single operation to be executed by the simulation.
type Scenario struct {
	Type     ScenarioType
	BookID   uuid.UUID
	ReaderID uuid.UUID
	IsError  bool   // True if this is an intentional error scenario
	Reason   string // Description of why this is an error scenario
}

// StateSnapshot holds cached state data to avoid mutex contention during scenario selection.
type StateSnapshot struct {
	books             int
	readers           int
	lending           int
	availableBooks    []string
	lentBooks         []string
	registeredReaders []string
	readersWithBooks  []string
	lastUpdated       time.Time
	mu                sync.RWMutex
}

// ScenarioSelector intelligently selects realistic scenarios based on current simulation state.
type ScenarioSelector struct {
	state    *SimulationState
	config   Config
	snapshot *StateSnapshot
}

// NewScenarioSelector creates a new scenario selector with the given state and configuration.
func NewScenarioSelector(state *SimulationState, config Config) *ScenarioSelector {
	selector := &ScenarioSelector{
		state:    state,
		config:   config,
		snapshot: &StateSnapshot{},
	}

	// Initialize with first snapshot
	selector.refreshSnapshot()

	return selector
}

// refreshSnapshot updates the cached state snapshot from the actual state.
func (s *ScenarioSelector) refreshSnapshot() {
	// Get all state data in a single batch to minimize mutex contention
	books, readers, lending := s.state.GetStats()
	availableBooks := s.state.GetAvailableBooks()
	lentBooks := s.state.GetLentBooks()
	registeredReaders := s.state.GetRegisteredReaders()
	readersWithBooks := s.state.GetReadersWithLentBooks()

	// Update snapshot under write lock
	s.snapshot.mu.Lock()
	s.snapshot.books = books
	s.snapshot.readers = readers
	s.snapshot.lending = lending
	s.snapshot.availableBooks = availableBooks
	s.snapshot.lentBooks = lentBooks
	s.snapshot.registeredReaders = registeredReaders
	s.snapshot.readersWithBooks = readersWithBooks
	s.snapshot.lastUpdated = time.Now()
	s.snapshot.mu.Unlock()
}

// ensureFreshSnapshot refreshes the snapshot if it's older than 100ms.
func (s *ScenarioSelector) ensureFreshSnapshot() {
	s.snapshot.mu.RLock()
	needsRefresh := time.Since(s.snapshot.lastUpdated) > 100*time.Millisecond
	s.snapshot.mu.RUnlock()

	if needsRefresh {
		s.refreshSnapshot()
	}
}

// getSnapshotData returns cached state data (refreshing if stale).
func (s *ScenarioSelector) getSnapshotData() (books, readers, lending int, availableBooks, lentBooks, registeredReaders, readersWithBooks []string) {
	s.ensureFreshSnapshot()

	s.snapshot.mu.RLock()
	defer s.snapshot.mu.RUnlock()

	return s.snapshot.books, s.snapshot.readers, s.snapshot.lending,
		s.snapshot.availableBooks, s.snapshot.lentBooks, s.snapshot.registeredReaders, s.snapshot.readersWithBooks
}

// SelectScenario chooses a realistic scenario based on current state and error injection probabilities.
func (s *ScenarioSelector) SelectScenario() Scenario {
	// Use cached snapshot data instead of hitting state mutex every time
	books, readers, lending, availableBooks, lentBooks, registeredReaders, readersWithBooks := s.getSnapshotData()

	// Check if we need to add books or readers to reach minimum thresholds
	if books < s.config.MinBooks {
		return s.createAddBookScenario()
	}

	if readers < s.config.MinReaders {
		return s.createRegisterReaderScenario()
	}

	// Determine primary scenario type based on current state and realistic weights
	scenarioWeights := s.calculateScenarioWeightsFromSnapshot(books, readers, lending, availableBooks, lentBooks)
	scenarioType := s.selectWeightedScenario(scenarioWeights)

	// Apply error injection if applicable
	if s.shouldInjectError(scenarioType) {
		return s.createErrorScenarioFromSnapshot(scenarioType, lentBooks, registeredReaders)
	}

	// Create normal scenario using cached data
	switch scenarioType {
	case ScenarioAddBook:
		return s.createAddBookScenario()
	case ScenarioRemoveBook:
		return s.createRemoveBookScenarioFromSnapshot(availableBooks)
	case ScenarioRegisterReader:
		return s.createRegisterReaderScenario()
	case ScenarioCancelReader:
		return s.createCancelReaderScenarioFromSnapshot(registeredReaders)
	case ScenarioLendBook:
		return s.createLendBookScenarioFromSnapshot(availableBooks, registeredReaders)
	case ScenarioReturnBook:
		return s.createReturnBookScenarioFromSnapshot(readersWithBooks)
	case ScenarioQueryBooks:
		return s.createQueryBooksScenarioFromSnapshot(registeredReaders)
	default:
		// Fallback to lending if no other scenario is suitable
		return s.createLendBookScenarioFromSnapshot(availableBooks, registeredReaders)
	}
}

// calculateScenarioWeightsFromSnapshot determines realistic weights using cached data (avoids mutex contention).
func (s *ScenarioSelector) calculateScenarioWeightsFromSnapshot(books, readers, _ int, availableBooks, lentBooks []string) map[ScenarioType]int {
	weights := make(map[ScenarioType]int)

	// Book management weights (library manager actions) - VERY RARE
	switch {
	case books > s.config.MaxBooks:
		weights[ScenarioRemoveBook] = 3 // Priority to remove excess books
		weights[ScenarioAddBook] = 0    // No adding when over max
	case books < s.config.MinBooks:
		weights[ScenarioAddBook] = 3    // Priority to reach minimum
		weights[ScenarioRemoveBook] = 0 // No removing when under min
	default:
		weights[ScenarioAddBook] = 1    // Very rare in normal operations
		weights[ScenarioRemoveBook] = 1 // Very rare in normal operations
	}

	// Reader management weights - VERY RARE
	switch {
	case readers < s.config.MinReaders:
		weights[ScenarioRegisterReader] = 5 // Priority to reach minimum readers
		weights[ScenarioCancelReader] = 0   // Don't cancel when below minimum
	case readers < s.config.MaxReaders:
		// Normal growth phase: favor registrations over cancellations
		weights[ScenarioRegisterReader] = 3 // Steady growth toward max
		weights[ScenarioCancelReader] = 1   // Minimal natural churn
	default:
		// At max readers, allow cancellations but no new registrations
		weights[ScenarioRegisterReader] = 0 // No new registrations at max
		weights[ScenarioCancelReader] = 3   // Higher to bring count down
	}

	// Lending weights (DOMINANT activity - 85-90% of operations) using cached data
	if len(availableBooks) > 0 && readers > 0 {
		weights[ScenarioLendBook] = 200 // DOMINANT: Most common operation
	}

	if len(lentBooks) > 0 {
		weights[ScenarioReturnBook] = 180 // DOMINANT: Second most common operation
		weights[ScenarioQueryBooks] = 10  // Occasional queries (2-3% of operations)
	}

	return weights
}

// selectWeightedScenario selects a scenario type based on weighted probabilities.
func (s *ScenarioSelector) selectWeightedScenario(weights map[ScenarioType]int) ScenarioType {
	totalWeight := 0
	for _, weight := range weights {
		totalWeight += weight
	}

	if totalWeight == 0 {
		return ScenarioLendBook // Fallback
	}

	r := rand.Intn(totalWeight) //nolint:gosec // Simulation code - weak random is acceptable
	currentWeight := 0

	for scenarioType, weight := range weights {
		currentWeight += weight
		if r < currentWeight {
			return scenarioType
		}
	}

	return ScenarioLendBook // Fallback
}

// shouldInjectError determines if this scenario should be an error scenario.
func (s *ScenarioSelector) shouldInjectError(scenarioType ScenarioType) bool {
	r := rand.Float64() * 100 //nolint:gosec // Simulation code - weak random is acceptable

	switch scenarioType {
	case ScenarioRemoveBook:
		return r < s.config.ErrorProbabilities.LibraryManagerBookConflict
	case ScenarioLendBook:
		return r < s.config.ErrorProbabilities.ReaderBorrowRemovedBook
	default:
		return r < s.config.ErrorProbabilities.IdempotentRepeat
	}
}

// createAddBookScenario creates a scenario to add a book to circulation.
func (s *ScenarioSelector) createAddBookScenario() Scenario {
	return Scenario{
		Type:   ScenarioAddBook,
		BookID: uuid.New(),
	}
}

// createRegisterReaderScenario creates a scenario to register a new reader.
func (s *ScenarioSelector) createRegisterReaderScenario() Scenario {
	return Scenario{
		Type:     ScenarioRegisterReader,
		ReaderID: uuid.New(),
	}
}

// createReturnBookScenarioFromSnapshot creates a return scenario using cached data (avoids mutex contention).
func (s *ScenarioSelector) createReturnBookScenarioFromSnapshot(readersWithBooks []string) Scenario {
	if len(readersWithBooks) == 0 {
		// No readers have books to return, create lending instead - but we need fresh data for that
		availableBooks := s.state.GetAvailableBooks()
		registeredReaders := s.state.GetRegisteredReaders()
		return s.createLendBookScenarioFromSnapshot(availableBooks, registeredReaders)
	}

	// Select a random reader who has books (from cached data)
	readerID := uuid.MustParse(readersWithBooks[rand.Intn(len(readersWithBooks))]) //nolint:gosec // Simulation code - weak random is acceptable

	// For performance, we'll need to check available books for return using the actual state
	// since this requires cross-referencing with pending returns (not cached)
	availableBooks := s.state.GetAvailableBooksForReturn(readerID)
	if len(availableBooks) == 0 {
		// This reader's books are all being returned, create lending instead
		availableBooks := s.state.GetAvailableBooks()
		registeredReaders := s.state.GetRegisteredReaders()
		return s.createLendBookScenarioFromSnapshot(availableBooks, registeredReaders)
	}

	bookID := uuid.MustParse(availableBooks[rand.Intn(len(availableBooks))]) //nolint:gosec // Simulation code - weak random is acceptable

	return Scenario{
		Type:     ScenarioReturnBook,
		BookID:   bookID,
		ReaderID: readerID,
	}
}

// Cached scenario creation methods (avoid mutex contention)

// createLendBookScenarioFromSnapshot creates a lending scenario using cached data.
func (s *ScenarioSelector) createLendBookScenarioFromSnapshot(availableBooks, registeredReaders []string) Scenario {
	if len(availableBooks) == 0 {
		return s.createAddBookScenario()
	}

	if len(registeredReaders) == 0 {
		return s.createRegisterReaderScenario()
	}

	bookID := uuid.MustParse(availableBooks[rand.Intn(len(availableBooks))])         //nolint:gosec
	readerID := uuid.MustParse(registeredReaders[rand.Intn(len(registeredReaders))]) //nolint:gosec

	return Scenario{
		Type:     ScenarioLendBook,
		BookID:   bookID,
		ReaderID: readerID,
	}
}

// createRemoveBookScenarioFromSnapshot creates a book removal scenario using cached data.
func (s *ScenarioSelector) createRemoveBookScenarioFromSnapshot(availableBooks []string) Scenario {
	if len(availableBooks) == 0 {
		return s.createAddBookScenario()
	}

	bookID := uuid.MustParse(availableBooks[rand.Intn(len(availableBooks))]) //nolint:gosec
	return Scenario{
		Type:   ScenarioRemoveBook,
		BookID: bookID,
	}
}

// createCancelReaderScenarioFromSnapshot creates a reader cancellation scenario using cached data.
func (s *ScenarioSelector) createCancelReaderScenarioFromSnapshot(registeredReaders []string) Scenario {
	if len(registeredReaders) == 0 {
		return s.createRegisterReaderScenario()
	}

	readerID := uuid.MustParse(registeredReaders[rand.Intn(len(registeredReaders))]) //nolint:gosec
	return Scenario{
		Type:     ScenarioCancelReader,
		ReaderID: readerID,
	}
}

// createQueryBooksScenarioFromSnapshot creates a query scenario using cached data.
func (s *ScenarioSelector) createQueryBooksScenarioFromSnapshot(registeredReaders []string) Scenario {
	var readerID uuid.UUID
	if len(registeredReaders) == 0 {
		readerID = uuid.New()
	} else {
		readerID = uuid.MustParse(registeredReaders[rand.Intn(len(registeredReaders))]) //nolint:gosec
	}

	return Scenario{
		Type:     ScenarioQueryBooks,
		ReaderID: readerID,
	}
}

// createErrorScenarioFromSnapshot creates error scenarios using cached data.
func (s *ScenarioSelector) createErrorScenarioFromSnapshot(scenarioType ScenarioType, lentBooks, registeredReaders []string) Scenario {
	switch scenarioType {
	case ScenarioRemoveBook:
		// Try to remove a book that is currently lent (should fail)
		if len(lentBooks) > 0 {
			bookID := uuid.MustParse(lentBooks[rand.Intn(len(lentBooks))]) //nolint:gosec
			return Scenario{
				Type:    ScenarioRemoveBook,
				BookID:  bookID,
				IsError: true,
				Reason:  "library manager tries to remove just-lent book",
			}
		}

	case ScenarioLendBook:
		// Try to lend a book that doesn't exist (simulate just-removed book)
		var readerID uuid.UUID
		if len(registeredReaders) > 0 {
			readerID = uuid.MustParse(registeredReaders[rand.Intn(len(registeredReaders))]) //nolint:gosec
		} else {
			readerID = uuid.New()
		}

		return Scenario{
			Type:     ScenarioLendBook,
			BookID:   uuid.New(), // Non-existent book
			ReaderID: readerID,
			IsError:  true,
			Reason:   "reader tries to borrow just-removed book",
		}
	}

	// Fallback - create idempotent scenario using cached data
	return s.createIdempotentScenarioFromSnapshot(scenarioType, lentBooks, registeredReaders)
}

// createIdempotentScenarioFromSnapshot creates idempotent scenarios using cached data.
func (s *ScenarioSelector) createIdempotentScenarioFromSnapshot(scenarioType ScenarioType, lentBooks, registeredReaders []string) Scenario {
	if scenarioType == ScenarioLendBook {
		// Try to lend a book that is already lent
		if len(lentBooks) > 0 {
			bookID := uuid.MustParse(lentBooks[rand.Intn(len(lentBooks))]) //nolint:gosec
			var readerID uuid.UUID
			if len(registeredReaders) > 0 {
				readerID = uuid.MustParse(registeredReaders[rand.Intn(len(registeredReaders))]) //nolint:gosec
			} else {
				readerID = uuid.New()
			}

			return Scenario{
				Type:     ScenarioLendBook,
				BookID:   bookID,
				ReaderID: readerID,
				IsError:  true,
				Reason:   "idempotent repeat - book already lent",
			}
		}
	}

	// Fallback to normal scenario
	return s.createLendBookScenarioFromSnapshot([]string{}, registeredReaders)
}
