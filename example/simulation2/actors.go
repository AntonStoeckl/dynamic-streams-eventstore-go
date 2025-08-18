// Package main implements an actor-based library simulation with realistic patron and librarian behaviors.
package main

import (
	"context"
	"fmt"
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
	Wishlist      []uuid.UUID // Books discovered during online browsing.
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
		Wishlist:      make([]uuid.UUID, 0, OnlineWishlistSize),
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

	// Secondary: Readers are more likely to visit if they have a wishlist.
	if len(r.Wishlist) > 0 {
		return decision < 0.8 // 80% chance if they have books in mind.
	}

	// Normal visiting pattern for readers with no books and no wishlist.
	switch {
	case decision < ChanceBrowseOnline:
		// Will browse online during the visit for wishlist.
		return true
	case decision < ChanceBrowseOnline+ChanceVisitDirectly:
		return true // A direct library visit.
	default:
		return false // Stay home today.
	}
}

// VisitLibrary simulates a complete library visit with natural flow.
func (r *ReaderActor) VisitLibrary(ctx context.Context, handlers *HandlerBundle) error {
	r.Location = Library
	r.Lifecycle = Active
	r.LastActivity = time.Now()

	hadBooksToReturn := len(r.BorrowedBooks) > 0

	// Phase 1: Return books if any (natural behavior - people return their books)
	if hadBooksToReturn {
		if err := r.returnBooks(ctx, handlers); err != nil {
			return err
		}
	}

	// Phase 2: Decide whether to browse for new books (natural probabilities)
	var shouldBrowse bool
	if hadBooksToReturn {
		// After returning books, many people browse for new ones (natural tendency)
		shouldBrowse = rand.Float64() < ChanceBorrowAfterReturn //nolint:gosec // Weak random OK for simulation
	} else {
		// Readers without books came to browse - normal browsing behavior
		browseChance := ChanceBrowseOnline + ChanceVisitDirectly // Normal browsing rate
		shouldBrowse = rand.Float64() < browseChance             //nolint:gosec // Weak random OK for simulation
	}

	if shouldBrowse {
		if err := r.browseAndBorrow(ctx, handlers); err != nil {
			return err
		}
	}

	// Phase 3: Leave the library
	r.Location = Home
	r.Lifecycle = AtHome
	r.LastActivity = time.Now()

	return nil
}

// returnBooks handles returning all or some of the reader's borrowed books.
func (r *ReaderActor) returnBooks(ctx context.Context, handlers *HandlerBundle) error {
	if len(r.BorrowedBooks) == 0 {
		return nil
	}

	// Decide how many books to return (fuzzy realistic behavior)
	var booksToReturn []uuid.UUID

	if rand.Float64() < ChanceReturnAll { //nolint:gosec // Weak random OK for simulation
		// Return all books
		booksToReturn = make([]uuid.UUID, len(r.BorrowedBooks))
		copy(booksToReturn, r.BorrowedBooks)
		r.BorrowedBooks = r.BorrowedBooks[:0] // Clear all
	} else {
		// Keep 1-2 books, return the rest
		keepCount := 1 + rand.Intn(2) //nolint:gosec // Weak random OK for simulation - Keep 1 or 2
		if keepCount >= len(r.BorrowedBooks) {
			return nil // Keep all books
		}

		returnCount := len(r.BorrowedBooks) - keepCount
		booksToReturn = make([]uuid.UUID, returnCount)
		copy(booksToReturn, r.BorrowedBooks[:returnCount])
		r.BorrowedBooks = r.BorrowedBooks[returnCount:] // Keep the rest
	}

	// Execute return commands through command handlers
	for _, bookID := range booksToReturn {
		if err := handlers.ExecuteReturnBook(ctx, bookID, r.ID); err != nil {
			return fmt.Errorf("failed to return book %s: %w", bookID, err)
		}
	}

	return nil
}

// browseAndBorrow handles book selection and borrowing.
func (r *ReaderActor) browseAndBorrow(ctx context.Context, handlers *HandlerBundle) error {
	// Don't borrow if the reader has too many books already
	availableSlots := MaxBooksPerReader - len(r.BorrowedBooks)
	if availableSlots <= 0 {
		return nil
	}

	// Decide how many books to borrow
	targetBooks := MinBooksPerVisit + rand.Intn(MaxBooksPerVisit-MinBooksPerVisit+1) //nolint:gosec // Weak random OK for simulation
	targetBooks = min(targetBooks, availableSlots)

	booksToBorrow := make([]uuid.UUID, 0, targetBooks)

	// Get current available books from simulation state.
	state := handlers.GetSimulationState()
	if state == nil {
		return nil // No state available, can't borrow books.
	}

	availableBooks := state.GetAvailableBooks()
	if len(availableBooks) == 0 {
		return nil // No books available to borrow.
	}

	// First, try to get wishlist books if they're still available.
	for _, bookID := range r.Wishlist {
		if len(booksToBorrow) >= targetBooks {
			break
		}

		// Check if the wishlist book is still available.
		if state.IsBookAvailable(bookID) {
			booksToBorrow = append(booksToBorrow, bookID)
		}
	}

	// If we need more books, browse the shelves.
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

		// Remove from available list to avoid re-selecting.
		availableBooks = append(availableBooks[:randomIndex], availableBooks[randomIndex+1:]...)
	}

	// Clear the wishlist after attempting to get those books
	r.Wishlist = r.Wishlist[:0]

	// Execute borrow commands through command handlers
	for _, bookID := range booksToBorrow {
		if err := handlers.ExecuteLendBook(ctx, bookID, r.ID); err != nil {
			return fmt.Errorf("failed to borrow book %s: %w", bookID, err)
		}

		// Optimistically add to borrowed books (will be corrected by state updates)
		r.BorrowedBooks = append(r.BorrowedBooks, bookID)
	}

	return nil
}

// ShouldCancelContract determines if this reader wants to cancel their library contract.
func (r *ReaderActor) ShouldCancelContract() bool {
	if r.Lifecycle != Registered && r.Lifecycle != AtHome {
		return false
	}

	// Only cancel if no books are borrowed
	if len(r.BorrowedBooks) > 0 {
		return false
	}

	// Random chance based on the tuning parameter
	return rand.Float64() < ReaderCancellationRate //nolint:gosec // Weak random OK for simulation
}

// =================================================================
// LIBRARIAN ACTOR - Models library staff managing inventory
// =================================================================

// LibrarianActor represents library staff managing the book collection.
type LibrarianActor struct {
	ID   uuid.UUID
	Role LibrarianRole // Acquisitions or Maintenance
}

// NewLibrarianActor creates a new librarian with the specified role.
func NewLibrarianActor(role LibrarianRole) *LibrarianActor {
	return &LibrarianActor{
		ID:   uuid.New(),
		Role: role,
	}
}

// Work performs librarian duties based on their role and current state using memory state for fast operations.
func (l *LibrarianActor) Work(ctx context.Context, handlers *HandlerBundle, state *SimulationState) error {
	switch l.Role {
	case Acquisitions:
		return l.manageBookAdditions(ctx, handlers, state)
	case Maintenance:
		return l.manageBookRemovals(ctx, handlers, state)
	default:
		return nil
	}
}

// manageBookAdditions adds new books to the collection when needed using fast memory state.
func (l *LibrarianActor) manageBookAdditions(ctx context.Context, handlers *HandlerBundle, state *SimulationState) error {
	// Get the current book count from memory state (fast operation)
	currentBooks := state.GetStats().TotalBooks

	if currentBooks < MinBooks {
		// Urgent: Add books quickly to reach minimum
		return l.addBooks(ctx, handlers, BookAdditionBatchSize)
	} else if currentBooks < MaxBooks {
		// Normal operations: Occasional additions
		if rand.Float64() < 0.1 { //nolint:gosec // Weak random OK for simulation - 10% chance
			return l.addBooks(ctx, handlers, 1)
		}
	}

	return nil
}

// manageBookRemovals removes old/damaged books when above maximum using fast memory state.
func (l *LibrarianActor) manageBookRemovals(ctx context.Context, handlers *HandlerBundle, state *SimulationState) error {
	// Get the current book count from memory state (fast operation)
	currentBooks := state.GetStats().TotalBooks

	if currentBooks > MaxBooks {
		// Need to reduce inventory
		return l.removeBooks(ctx, handlers, BookRemovalBatchSize)
	} else if currentBooks > MinBooks {
		// Normal maintenance
		if rand.Float64() < LibrarianMaintenanceChance { //nolint:gosec // Weak random OK for simulation
			return l.removeBooks(ctx, handlers, 1)
		}
	}

	return nil
}

// addBooks adds the specified number of books to the collection.
func (l *LibrarianActor) addBooks(ctx context.Context, handlers *HandlerBundle, count int) error {
	for i := 0; i < count; i++ {
		bookID := uuid.New()

		// Execute the add book command
		if err := handlers.ExecuteAddBook(ctx, bookID); err != nil {
			return fmt.Errorf("failed to add book %s: %w", bookID, err)
		}
	}

	return nil
}

// removeBooks removes the specified number of available books.
func (l *LibrarianActor) removeBooks(ctx context.Context, handlers *HandlerBundle, count int) error {
	// Get available (not lent) books from simulation state.
	state := handlers.GetSimulationState()
	if state == nil {
		return fmt.Errorf("no simulation state available for book removal")
	}

	availableBooks := state.GetAvailableBooks()
	if len(availableBooks) == 0 {
		return nil // No available books to remove.
	}

	// Remove up to the requested count, but don't exceed available books.
	actualCount := min(count, len(availableBooks))

	for i := 0; i < actualCount; i++ {
		// Select a random available book to remove.
		randomIndex := rand.Intn(len(availableBooks)) //nolint:gosec // Weak random OK for simulation
		bookID := availableBooks[randomIndex]

		// Execute the remove book command.
		if err := handlers.ExecuteRemoveBook(ctx, bookID); err != nil {
			return fmt.Errorf("failed to remove book %s: %w", bookID, err)
		}

		// Remove from local list to avoid selecting it again.
		availableBooks = append(availableBooks[:randomIndex], availableBooks[randomIndex+1:]...)
	}

	return nil
}
