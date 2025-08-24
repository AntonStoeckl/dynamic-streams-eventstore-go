// Package main implements an actor-based library simulation with realistic patron and librarian behaviors.
package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/google/uuid"
)

// =================================================================
// READER ACTOR - Models real library patron behavior
// =================================================================

// ReaderLifecycle represents the current state of a reader in the system.
type ReaderLifecycle int

const (
	NotRegistered ReaderLifecycle = iota // New reader, not yet registered.
	Registered                           // Has library card, can visit.
	Active                               // Currently visiting the library.
	AtHome                               // Has card, not visiting today.
	Canceled                             // Contract terminated.
)

// ReaderLocation tracks where the reader is currently active.
type ReaderLocation int

const (
	Home ReaderLocation = iota
	Library
)

// ReaderActor represents a single library patron with realistic behavior patterns.
type ReaderActor struct {
	ID            uuid.UUID
	Lifecycle     ReaderLifecycle
	Location      ReaderLocation
	BorrowedBooks []uuid.UUID // Currently borrowed books (max 10).
	LastActivity  time.Time   // When this reader last did something.

	// Persona characteristics (future enhancement).
	Persona ReaderPersona // Casual, PowerUser, Student, etc.
}

// NewReaderActor creates a new reader actor with the given persona.
func NewReaderActor(persona ReaderPersona) *ReaderActor {
	return &ReaderActor{
		ID:            uuid.New(),
		Lifecycle:     NotRegistered, // Starts unregistered.
		Location:      Home,
		BorrowedBooks: make([]uuid.UUID, 0, MaxBooksPerReader),
		LastActivity:  time.Now(),
		Persona:       persona,
	}
}

// ShouldVisitLibrary determines if this reader wants to visit the library today.
func (r *ReaderActor) ShouldVisitLibrary() bool {
	if r.Lifecycle != Registered && r.Lifecycle != AtHome {
		return false // Not in a state to visit.
	}

	decision := rand.Float64() //nolint:gosec // Weak random OK for simulation

	// PRIORITY: Readers with borrowed books are MUCH more likely to visit (to return them).
	if len(r.BorrowedBooks) > 0 {
		return decision < 0.9 // 90% chance if they have books to return.
	}

	// Normal visiting pattern for readers without books to return.
	return decision < ChanceVisitDirectly // Direct library visit to browse books.
}

// VisitLibrary simulates a complete library visit with natural flow.
// Returns the number of operations performed and any error.
func (r *ReaderActor) VisitLibrary(ctx context.Context, handlers *HandlerBundle) (int, error) {
	r.Location = Library
	r.Lifecycle = Active
	r.LastActivity = time.Now()

	totalOperations := 0
	hadBooksToReturn := len(r.BorrowedBooks) > 0

	// Phase 1: Return books if any (natural behavior - people return their books)
	if hadBooksToReturn {
		returnOps, err := r.returnBooks(ctx, handlers)
		if err != nil {
			// Only propagate technical errors (timeouts, cancellations)
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return 0, err
			}
			// Business failures are handled gracefully - continue with the visit
		}
		totalOperations += returnOps
	}

	// Phase 2: Decide whether to browse for new books (natural probabilities)
	var shouldBrowse bool
	if hadBooksToReturn {
		// After returning books, many people browse for new ones (natural tendency)
		shouldBrowse = rand.Float64() < ChanceBorrowAfterReturn //nolint:gosec // Weak random OK for simulation
	} else {
		// Readers without books came to browse - they definitely want to browse
		shouldBrowse = true
	}

	if shouldBrowse {
		borrowOps, err := r.browseAndBorrow(ctx, handlers)
		if err != nil {
			// Only propagate technical errors (timeouts, cancellations)
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return totalOperations, err
			}
			// Business failures are handled gracefully - continue with the visit
		}
		totalOperations += borrowOps
	}

	// Phase 3: Leave the library
	r.Location = Home
	r.Lifecycle = AtHome
	r.LastActivity = time.Now()

	return totalOperations, nil
}

// returnBooks handles returning all or some of the reader's borrowed books.
// Returns the number of operations performed and any error.
func (r *ReaderActor) returnBooks(ctx context.Context, handlers *HandlerBundle) (int, error) {
	if len(r.BorrowedBooks) == 0 {
		return 0, nil
	}

	// Decide how many books to return (fuzzy realistic behavior)
	var booksToReturn []uuid.UUID

	// Decide what to return WITHOUT modifying state yet
	if rand.Float64() < ChanceReturnAll { //nolint:gosec // Weak random OK for simulation
		// Return all books
		booksToReturn = make([]uuid.UUID, len(r.BorrowedBooks))
		copy(booksToReturn, r.BorrowedBooks)
		// DON'T clear yet: r.BorrowedBooks = r.BorrowedBooks[:0]
	} else {
		// Keep 1-2 books, return the rest
		keepCount := 1 + rand.Intn(2) //nolint:gosec // Weak random OK for simulation - Keep 1 or 2
		if keepCount >= len(r.BorrowedBooks) {
			return 0, nil // Keep all books
		}

		returnCount := len(r.BorrowedBooks) - keepCount
		booksToReturn = make([]uuid.UUID, returnCount)
		copy(booksToReturn, r.BorrowedBooks[:returnCount])
		// DON'T modify yet: r.BorrowedBooks = r.BorrowedBooks[returnCount:]
	}

	// Execute return commands through command handlers - handle failures gracefully
	operationCount := 0
	successfulReturns := make([]uuid.UUID, 0, len(booksToReturn))

	for _, bookID := range booksToReturn {
		if err := handlers.ExecuteReturnBook(ctx, bookID, r.ID); err != nil {
			// Some returns might fail if the state was inconsistent - skip and continue
			continue
		}
		operationCount++
		successfulReturns = append(successfulReturns, bookID)
	}

	// NOW update state based ONLY on what actually succeeded
	updatedBorrowedBooks := make([]uuid.UUID, 0, len(r.BorrowedBooks))
	for _, bookID := range r.BorrowedBooks {
		// Keep books that were NOT successfully returned
		wasReturned := false
		for _, returnedID := range successfulReturns {
			if bookID == returnedID {
				wasReturned = true
				break
			}
		}
		if !wasReturned {
			updatedBorrowedBooks = append(updatedBorrowedBooks, bookID)
		}
	}
	r.BorrowedBooks = updatedBorrowedBooks

	// Never return error for business failures - system must continue operating
	return operationCount, nil
}

// browseAndBorrow handles book selection and borrowing.
// Returns the number of operations performed and any error.
func (r *ReaderActor) browseAndBorrow(ctx context.Context, handlers *HandlerBundle) (int, error) {
	// Don't borrow if the reader has too many books already
	availableSlots := MaxBooksPerReader - len(r.BorrowedBooks)
	if availableSlots <= 0 {
		return 0, nil
	}

	// Decide how many books to borrow
	targetBooks := MinBooksPerVisit + rand.Intn(MaxBooksPerVisit-MinBooksPerVisit+1) //nolint:gosec // Weak random OK for simulation
	targetBooks = min(targetBooks, availableSlots)

	booksToBorrow := make([]uuid.UUID, 0, targetBooks)

	// Get current available books from the simulation state.
	state := handlers.GetSimulationState()
	if state == nil {
		return 0, nil // No state available, can't borrow books.
	}

	availableBooks := state.GetAvailableBooks()
	if len(availableBooks) == 0 {
		return 0, nil // No books available to borrow.
	}

	// Browse the shelves for interesting books.
	for len(booksToBorrow) < targetBooks && len(availableBooks) > 0 {
		// Randomly select from available books.
		randomIndex := rand.Intn(len(availableBooks)) //nolint:gosec // Weak random OK for simulation
		bookID := availableBooks[randomIndex]

		// Avoid duplicates.
		duplicate := false
		for _, existingBook := range booksToBorrow {
			if existingBook == bookID {
				duplicate = true
				break
			}
		}

		if !duplicate {
			booksToBorrow = append(booksToBorrow, bookID)
		}

		// Remove from the available list to avoid re-selecting.
		availableBooks = append(availableBooks[:randomIndex], availableBooks[randomIndex+1:]...)
	}

	// Execute borrow commands through command handlers - handle failures gracefully
	operationCount := 0
	successfulBorrows := make([]uuid.UUID, 0, len(booksToBorrow))

	for _, bookID := range booksToBorrow {
		if err := handlers.ExecuteLendBook(ctx, bookID, r.ID); err != nil {
			// Race conditions and business rule violations are EXPECTED in a busy library
			// Don't fail the entire visit - just skip this book and try others
			continue
		}
		operationCount++
		successfulBorrows = append(successfulBorrows, bookID)
	}

	// Only update reader state with books that were ACTUALLY borrowed successfully
	r.BorrowedBooks = append(r.BorrowedBooks, successfulBorrows...)

	// Never return error for business failures - system must continue operating
	return operationCount, nil
}

// ShouldCancelContract determines if this reader wants to cancel their library contract using dynamic probability.
func (r *ReaderActor) ShouldCancelContract(totalReaders int) bool {
	if r.Lifecycle != Registered && r.Lifecycle != AtHome {
		return false
	}

	// Only cancel if no books are borrowed
	if len(r.BorrowedBooks) > 0 {
		return false
	}

	// Use dynamic probability based on the current reader population
	_, cancelProb := calculateReaderProbabilities(totalReaders)
	return rand.Float64() < cancelProb //nolint:gosec // Weak random OK for simulation
}

// =================================================================
// LIBRARIAN ACTOR - Models library staff managing inventory
// =================================================================

// LibrarianActor represents library staff managing the book collection.
type LibrarianActor struct {
	ID       uuid.UUID
	Role     LibrarianRole // Acquisitions or Maintenance
	Momentum float64       // -1.0 (removing trend) to +1.0 (adding trend)
}

// NewLibrarianActor creates a new librarian with the specified role.
func NewLibrarianActor(role LibrarianRole) *LibrarianActor {
	return &LibrarianActor{
		ID:       uuid.New(),
		Role:     role,
		Momentum: 0.0, // Start with neutral momentum
	}
}

// Work performs librarian duties using a dynamic probability system with momentum, waves, and bursts.
// Returns the number of operations performed and any error.
func (l *LibrarianActor) Work(ctx context.Context, handlers *HandlerBundle, state *SimulationState, cycleNum int64) (int, error) {
	// Get current book count for dynamic probability calculations
	currentBooks := state.GetStats().TotalBooks

	// Check for burst mode first (occasional extremes)
	if rand.Float64() < BurstChance { //nolint:gosec // Weak random OK for simulation
		return l.handleBurstMode(ctx, handlers, currentBooks)
	}

	// Calculate dynamic probabilities based on current position, momentum, and waves
	addProb, removeProb := l.calculateDynamicProbabilities(currentBooks, cycleNum)

	// Apply role-based logic with dynamic probabilities
	switch l.Role {
	case Acquisitions:
		if rand.Float64() < addProb { //nolint:gosec // Weak random OK for simulation
			operationCount, err := l.addBooks(ctx, handlers, 1)
			l.updateMomentum(operationCount > 0, true) // true = addition
			return operationCount, err
		}
	case Maintenance:
		if rand.Float64() < removeProb { //nolint:gosec // Weak random OK for simulation
			operationCount, err := l.removeBooks(ctx, handlers, 1)
			l.updateMomentum(operationCount > 0, false) // false = removal
			return operationCount, err
		}
	}

	// Decay momentum even when not working
	l.updateMomentum(false, true) // neutral update

	return 0, nil
}

// addBooks adds the specified number of books to the collection.
// Returns the number of operations performed and any error.
func (l *LibrarianActor) addBooks(ctx context.Context, handlers *HandlerBundle, count int) (int, error) {
	operationCount := 0
	for i := 0; i < count; i++ {
		bookID := uuid.New()

		// Execute the add book command - continue on failure like readers do
		if err := handlers.ExecuteAddBook(ctx, bookID); err != nil {
			continue // Skip failed additions, try the next book
		}
		operationCount++
	}

	return operationCount, nil
}

// removeBooks removes the specified number of available books.
// Returns the number of operations performed and any error.
func (l *LibrarianActor) removeBooks(ctx context.Context, handlers *HandlerBundle, count int) (int, error) {
	// Get available (not lent) books from the simulation state.
	state := handlers.GetSimulationState()
	if state == nil {
		return 0, fmt.Errorf("no simulation state available for book removal")
	}

	availableBooks := state.GetAvailableBooks()
	if len(availableBooks) == 0 {
		return 0, nil // No available books to remove.
	}

	// Remove up to the requested count but don't exceed available books.
	actualCount := min(count, len(availableBooks))

	operationCount := 0
	for i := 0; i < actualCount; i++ {
		// Select a random available book to remove.
		randomIndex := rand.Intn(len(availableBooks)) //nolint:gosec // Weak random OK for simulation
		bookID := availableBooks[randomIndex]

		// Execute the remove book command - continue on failure like readers do
		if err := handlers.ExecuteRemoveBook(ctx, bookID); err != nil {
			continue // Skip failed removals, try the next book
		}
		operationCount++

		// Remove from the local list to avoid selecting it again.
		availableBooks = append(availableBooks[:randomIndex], availableBooks[randomIndex+1:]...)
	}

	return operationCount, nil
}

// calculateDynamicProbabilities computes add/remove probabilities using a multi-layered system.
func (l *LibrarianActor) calculateDynamicProbabilities(currentBooks int, cycleNum int64) (addProb, removeProb float64) {
	// 1. Distance-based base probability (your original idea!)
	bookRange := float64(MaxBooks - MinBooks)
	position := float64(currentBooks-MinBooks) / bookRange // 0.0 to 1.0

	// Inverse relationship: near min = high add rate, near max = high remove rate
	baseAddProb := (1.0 - position) * LibrarianWorkProbability
	baseRemoveProb := position * LibrarianWorkProbability

	// 2. Random walk component (prevents equilibrium)
	randomWalk := (rand.Float64() - 0.5) * BaseRandomness //nolint:gosec // Weak random OK for simulation
	addProb = baseAddProb + randomWalk
	removeProb = baseRemoveProb - randomWalk // Opposite direction

	// 3. Wave patterns (natural fluctuations)
	wavePhase := float64(cycleNum) * 0.05 // Slow oscillation
	waveFactor := math.Sin(wavePhase) * WaveAmplitude
	addProb *= 1.0 + waveFactor
	removeProb *= 1.0 - waveFactor // Opposite wave

	// 4. Momentum influence (trend following)
	addProb *= 1.0 + l.Momentum*MomentumInfluence
	removeProb *= 1.0 - l.Momentum*MomentumInfluence

	// 5. Clamp to valid probability range
	addProb = math.Max(0.0, math.Min(1.0, addProb))
	removeProb = math.Max(0.0, math.Min(1.0, removeProb))

	return addProb, removeProb
}

// handleBurstMode manages occasional extreme book management for full range usage.
func (l *LibrarianActor) handleBurstMode(ctx context.Context, handlers *HandlerBundle, currentBooks int) (int, error) {
	midpoint := (MinBooks + MaxBooks) / 2

	if currentBooks < midpoint {
		// Acquisition burst - add many books quickly
		operationCount, err := l.addBooks(ctx, handlers, AcquisitionBurstSize)
		l.updateMomentum(operationCount > 0, true) // Strong positive momentum
		return operationCount, err
	}

	// Clearance burst - remove many books quickly
	operationCount, err := l.removeBooks(ctx, handlers, ClearanceBurstSize)
	l.updateMomentum(operationCount > 0, false) // Strong negative momentum
	return operationCount, err
}

// updateMomentum adjusts librarian momentum based on recent actions.
func (l *LibrarianActor) updateMomentum(actionSuccessful, isAddition bool) {
	if actionSuccessful {
		if isAddition {
			// Successful addition increases positive momentum
			l.Momentum = l.Momentum*MomentumDecay + 0.2
		} else {
			// Successful removal increases negative momentum
			l.Momentum = l.Momentum*MomentumDecay - 0.2
		}
	} else {
		// No action or failure - just decay momentum toward zero
		l.Momentum *= MomentumDecay
	}

	// Clamp momentum to valid range
	l.Momentum = math.Max(-1.0, math.Min(1.0, l.Momentum))
}

// calculateReaderProbabilities computes registration/cancellation probabilities based on current population.
func calculateReaderProbabilities(currentReaders int) (registerProb, cancelProb float64) {
	// Calculate position within the reader range, allowing values outside [0,1] for urgency
	readerRange := float64(MaxReaders - MinReaders) // 1000 reader range
	position := float64(currentReaders-MinReaders) / readerRange

	// Don't clamp - allow negative/above 1.0 for urgent corrections
	// position < 0.0 = below minimum (urgent registration needed)
	// position > 1.0 = above maximum (urgent cancellation needed)

	// Distance-based probabilities with urgency multiplier
	// Below min (negative position): extra high register, zero cancel
	// Above max (>1.0 position): zero register, extra high cancel
	baseRegisterProb := math.Max(0.0, (1.0-position)*BaseRegisterRate)
	baseCancelProb := math.Max(0.0, position*BaseCancelRate)

	// Add random walk component (prevents equilibrium)
	randomWalk := (rand.Float64() - 0.5) * ReaderRandomness //nolint:gosec // Weak random OK for simulation
	registerProb = baseRegisterProb + randomWalk
	cancelProb = baseCancelProb - randomWalk // Opposite direction

	// Clamp to valid probability range
	registerProb = math.Max(0.0, math.Min(1.0, registerProb))
	cancelProb = math.Max(0.0, math.Min(1.0, cancelProb))

	return registerProb, cancelProb
}
