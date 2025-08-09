package main

import (
	"math/rand"

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

// ScenarioSelector intelligently selects realistic scenarios based on current simulation state.
type ScenarioSelector struct {
	state  *SimulationState
	config Config
}

// NewScenarioSelector creates a new scenario selector with the given state and configuration.
func NewScenarioSelector(state *SimulationState, config Config) *ScenarioSelector {
	return &ScenarioSelector{
		state:  state,
		config: config,
	}
}

// SelectScenario chooses a realistic scenario based on current state and error injection probabilities.
func (s *ScenarioSelector) SelectScenario() Scenario {
	books, readers, lending := s.state.GetStats()

	// Check if we need to add books or readers to reach minimum thresholds
	if books < s.config.MinBooks {
		return s.createAddBookScenario()
	}

	if readers < s.config.MinReaders {
		return s.createRegisterReaderScenario()
	}

	// Determine primary scenario type based on current state and realistic weights
	scenarioWeights := s.calculateScenarioWeights(books, readers, lending)
	scenarioType := s.selectWeightedScenario(scenarioWeights)

	// Apply error injection if applicable
	if s.shouldInjectError(scenarioType) {
		return s.createErrorScenario(scenarioType)
	}

	// Create normal scenario
	switch scenarioType {
	case ScenarioAddBook:
		return s.createAddBookScenario()
	case ScenarioRemoveBook:
		return s.createRemoveBookScenario()
	case ScenarioRegisterReader:
		return s.createRegisterReaderScenario()
	case ScenarioCancelReader:
		return s.createCancelReaderScenario()
	case ScenarioLendBook:
		return s.createLendBookScenario()
	case ScenarioReturnBook:
		return s.createReturnBookScenario()
	case ScenarioQueryBooks:
		return s.createQueryBooksScenario()
	default:
		// Fallback to lending if no other scenario is suitable
		return s.createLendBookScenario()
	}
}

// calculateScenarioWeights determines realistic weights for different scenarios based on current state.
func (s *ScenarioSelector) calculateScenarioWeights(books, readers, _ int) map[ScenarioType]int {
	weights := make(map[ScenarioType]int)

	// Book management weights (library manager actions) - VERY RARE
	if books > s.config.MaxBooks {
		weights[ScenarioRemoveBook] = 3 // Priority to remove excess books
		weights[ScenarioAddBook] = 0    // No adding when over max
	} else if books < s.config.MinBooks {
		weights[ScenarioAddBook] = 3    // Priority to reach minimum
		weights[ScenarioRemoveBook] = 0 // No removing when under min
	} else {
		weights[ScenarioAddBook] = 1    // Very rare in normal operations
		weights[ScenarioRemoveBook] = 1 // Very rare in normal operations
	}

	// Reader management weights - VERY RARE
	if readers < s.config.MinReaders {
		weights[ScenarioRegisterReader] = 5 // Priority to reach minimum readers
		weights[ScenarioCancelReader] = 0   // Don't cancel when below minimum
	} else if readers < s.config.MaxReaders {
		// Normal growth phase: favor registrations over cancellations
		weights[ScenarioRegisterReader] = 3 // Steady growth toward max
		weights[ScenarioCancelReader] = 1   // Minimal natural churn
	} else {
		// At max readers, allow cancellations but no new registrations
		weights[ScenarioRegisterReader] = 0 // No new registrations at max
		weights[ScenarioCancelReader] = 3   // Higher to bring count down
	}

	// Lending weights (DOMINANT activity - 85-90% of operations)
	availableBooks := s.state.GetAvailableBooks()
	lentBooks := s.state.GetLentBooks()

	if len(availableBooks) > 0 && len(s.state.GetRegisteredReaders()) > 0 {
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

// createErrorScenario creates an intentional error scenario for testing.
func (s *ScenarioSelector) createErrorScenario(scenarioType ScenarioType) Scenario {
	switch scenarioType {
	case ScenarioRemoveBook:
		// Try to remove a book that is currently lent (should fail)
		lentBooks := s.state.GetLentBooks()
		if len(lentBooks) > 0 {
			bookID := uuid.MustParse(lentBooks[rand.Intn(len(lentBooks))]) //nolint:gosec // Simulation code - weak random is acceptable
			return Scenario{
				Type:    ScenarioRemoveBook,
				BookID:  bookID,
				IsError: true,
				Reason:  "library manager tries to remove just-lent book",
			}
		}

	case ScenarioLendBook:
		// Try to lend a book that doesn't exist (simulate just-removed book)
		return Scenario{
			Type:     ScenarioLendBook,
			BookID:   uuid.New(), // Non-existent book
			ReaderID: s.getRandomReaderID(),
			IsError:  true,
			Reason:   "reader tries to borrow just-removed book",
		}

	default:
		// Idempotent repeat - just repeat the same operation
		return s.createIdempotentScenario(scenarioType)
	}

	// Fallback to normal scenario if error creation fails
	return s.createNormalScenario(scenarioType)
}

// createIdempotentScenario creates a scenario that should be idempotent (no-op).
func (s *ScenarioSelector) createIdempotentScenario(scenarioType ScenarioType) Scenario {
	switch scenarioType {
	case ScenarioAddBook:
		// Try to add a book that already exists
		availableBooks := s.state.GetAvailableBooks()
		if len(availableBooks) > 0 {
			bookID := uuid.MustParse(availableBooks[rand.Intn(len(availableBooks))]) //nolint:gosec // Simulation code - weak random is acceptable
			return Scenario{
				Type:    ScenarioAddBook,
				BookID:  bookID,
				IsError: true,
				Reason:  "idempotent repeat - book already exists",
			}
		}

	case ScenarioLendBook:
		// Try to lend a book that is already lent
		lentBooks := s.state.GetLentBooks()
		if len(lentBooks) > 0 {
			bookID := uuid.MustParse(lentBooks[rand.Intn(len(lentBooks))]) //nolint:gosec // Simulation code - weak random is acceptable
			return Scenario{
				Type:     ScenarioLendBook,
				BookID:   bookID,
				ReaderID: s.getRandomReaderID(),
				IsError:  true,
				Reason:   "idempotent repeat - book already lent",
			}
		}
	}

	// Fallback to normal scenario
	return s.createNormalScenario(scenarioType)
}

// createNormalScenario creates a normal (non-error) scenario.
func (s *ScenarioSelector) createNormalScenario(scenarioType ScenarioType) Scenario {
	switch scenarioType {
	case ScenarioAddBook:
		return s.createAddBookScenario()
	case ScenarioRemoveBook:
		return s.createRemoveBookScenario()
	case ScenarioRegisterReader:
		return s.createRegisterReaderScenario()
	case ScenarioCancelReader:
		return s.createCancelReaderScenario()
	case ScenarioLendBook:
		return s.createLendBookScenario()
	case ScenarioReturnBook:
		return s.createReturnBookScenario()
	case ScenarioQueryBooks:
		return s.createQueryBooksScenario()
	default:
		return s.createLendBookScenario()
	}
}

// createAddBookScenario creates a scenario to add a book to circulation.
func (s *ScenarioSelector) createAddBookScenario() Scenario {
	return Scenario{
		Type:   ScenarioAddBook,
		BookID: uuid.New(),
	}
}

// createRemoveBookScenario creates a scenario to remove a book from circulation.
func (s *ScenarioSelector) createRemoveBookScenario() Scenario {
	availableBooks := s.state.GetAvailableBooks()
	if len(availableBooks) == 0 {
		// No available books to remove, create add book instead
		return s.createAddBookScenario()
	}

	bookID := uuid.MustParse(availableBooks[rand.Intn(len(availableBooks))]) //nolint:gosec // Simulation code - weak random is acceptable
	return Scenario{
		Type:   ScenarioRemoveBook,
		BookID: bookID,
	}
}

// createRegisterReaderScenario creates a scenario to register a new reader.
func (s *ScenarioSelector) createRegisterReaderScenario() Scenario {
	return Scenario{
		Type:     ScenarioRegisterReader,
		ReaderID: uuid.New(),
	}
}

// createCancelReaderScenario creates a scenario to cancel a reader's contract.
func (s *ScenarioSelector) createCancelReaderScenario() Scenario {
	readers := s.state.GetRegisteredReaders()
	if len(readers) == 0 {
		// No readers to cancel, register one instead
		return s.createRegisterReaderScenario()
	}

	// Select a random reader to cancel
	// In real life, we'd prefer readers without active lendings, but
	// the business logic should handle the case where they have books
	readerID := uuid.MustParse(readers[rand.Intn(len(readers))]) //nolint:gosec // Simulation code - weak random is acceptable
	return Scenario{
		Type:     ScenarioCancelReader,
		ReaderID: readerID,
	}
}

// createLendBookScenario creates a scenario to lend a book to a reader.
func (s *ScenarioSelector) createLendBookScenario() Scenario {
	availableBooks := s.state.GetAvailableBooks()
	if len(availableBooks) == 0 {
		// No books available, create add book instead
		return s.createAddBookScenario()
	}

	readers := s.state.GetRegisteredReaders()
	if len(readers) == 0 {
		// No readers registered, register one instead
		return s.createRegisterReaderScenario()
	}

	bookID := uuid.MustParse(availableBooks[rand.Intn(len(availableBooks))]) //nolint:gosec // Simulation code - weak random is acceptable
	readerID := uuid.MustParse(readers[rand.Intn(len(readers))])             //nolint:gosec // Simulation code - weak random is acceptable

	return Scenario{
		Type:     ScenarioLendBook,
		BookID:   bookID,
		ReaderID: readerID,
	}
}

// createReturnBookScenario creates a scenario to return a book from a reader using actor-aware logic.
// This ensures that only readers who actually have borrowed books can return them, eliminating
// unrealistic concurrency conflicts where multiple workers try to return the same book.
func (s *ScenarioSelector) createReturnBookScenario() Scenario {
	// Step 1: Find readers who have books to return (actor-aware approach)
	readersWithBooks := s.state.GetReadersWithLentBooks()
	if len(readersWithBooks) == 0 {
		// No readers have books to return, create lending instead
		return s.createLendBookScenario()
	}

	// Step 2: Try multiple readers to find one with available books to return
	// (books not already being returned by another worker)
	maxAttempts := len(readersWithBooks)
	if maxAttempts > 10 {
		maxAttempts = 10 // Limit attempts to avoid excessive searching
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Select a random reader who has books
		readerID := uuid.MustParse(readersWithBooks[rand.Intn(len(readersWithBooks))]) //nolint:gosec // Simulation code - weak random is acceptable

		// Step 3: Get books that THIS specific reader has borrowed AND are not being returned
		availableBooks := s.state.GetAvailableBooksForReturn(readerID)
		if len(availableBooks) == 0 {
			// This reader's books are all being returned or were just returned, try another reader
			continue
		}

		// Step 4: Select one of their available books to return
		bookID := uuid.MustParse(availableBooks[rand.Intn(len(availableBooks))]) //nolint:gosec // Simulation code - weak random is acceptable

		// Successfully found an available book - return this scenario
		// Note: Reservation happens in executeRequest to be closer to actual execution
		return Scenario{
			Type:     ScenarioReturnBook,
			BookID:   bookID,
			ReaderID: readerID, // Guaranteed to be the correct reader who borrowed this book
		}
	}

	// All attempts failed - all books are being returned, create lending instead
	return s.createLendBookScenario()
}

// createQueryBooksScenario creates a scenario to query books lent by a reader.
func (s *ScenarioSelector) createQueryBooksScenario() Scenario {
	readerID := s.getRandomReaderID()
	return Scenario{
		Type:     ScenarioQueryBooks,
		ReaderID: readerID,
	}
}

// getRandomReaderID returns a random registered reader ID, or generates new one if none exist.
func (s *ScenarioSelector) getRandomReaderID() uuid.UUID {
	readers := s.state.GetRegisteredReaders()
	if len(readers) == 0 {
		return uuid.New() // Will trigger reader registration
	}

	return uuid.MustParse(readers[rand.Intn(len(readers))]) //nolint:gosec // Simulation code - weak random is acceptable
}

// getReaderForBook returns the reader ID who has borrowed the given book.
func (s *ScenarioSelector) getReaderForBook(bookID uuid.UUID) uuid.UUID {
	s.state.mu.RLock()
	defer s.state.mu.RUnlock()

	if readerIDStr, exists := s.state.lending[bookID.String()]; exists {
		return uuid.MustParse(readerIDStr)
	}

	// Fallback to random reader (shouldn't happen in normal scenarios)
	return s.getRandomReaderID()
}
