package main

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/query/booksincirculation"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/query/bookslentout"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/query/registeredreaders"
)

// =================================================================
// SIMULATION STATE - Fast in-memory lookups for actor decisions
// =================================================================

// BookState represents the current state of a book in the system.
type BookState struct {
	ID        uuid.UUID
	Available bool      // true if not lent out.
	LentTo    uuid.UUID // ReaderID if lent, zero UUID if available.
	AddedAt   time.Time
}

// ReaderState represents the current state of a reader in the system.
type ReaderState struct {
	ID            uuid.UUID
	Name          string
	Lifecycle     ReaderLifecycle
	BorrowedBooks []uuid.UUID // Books currently borrowed by this reader.
	RegisteredAt  time.Time
	LastActivity  time.Time
}

// SimulationState manages the current state of the library system for fast actor decisions.
// This provides the fast lookups that actors need without hitting the EventStore every time.
type SimulationState struct {
	mu sync.RWMutex

	// Book tracking.
	books            map[uuid.UUID]*BookState
	availableBookIDs []uuid.UUID // Pre-computed for fast browsing.
	totalBooks       int

	// Reader tracking.
	readers           map[uuid.UUID]*ReaderState
	registeredReaders []uuid.UUID // Pre-computed list.
	totalReaders      int

	// Lending relationships (book -> reader mapping).
	lendingMap          map[uuid.UUID]uuid.UUID   // BookID -> ReaderID.
	readerBooksMap      map[uuid.UUID][]uuid.UUID // ReaderID -> [BookIDs].
	totalActiveLendings int

	// Cache refresh tracking.
	lastRefresh       time.Time
	refreshInProgress bool

	// Statistics for monitoring.
	stats SimulationStats
}

// SimulationStats holds key metrics for monitoring and auto-tuning.
type SimulationStats struct {
	TotalBooks        int
	AvailableBooks    int
	BooksLentOut      int
	TotalReaders      int
	ActiveLendings    int
	CompletedLendings int64 // Total returns since start.

	// Population dynamics.
	ReaderRegistrations int64
	ReaderCancellations int64
	BookAdditions       int64
	BookRemovals        int64
	StateRefreshes      int64
}

// NewSimulationState creates a new simulation state manager.
func NewSimulationState() *SimulationState {
	return &SimulationState{
		books:             make(map[uuid.UUID]*BookState),
		availableBookIDs:  make([]uuid.UUID, 0),
		readers:           make(map[uuid.UUID]*ReaderState),
		registeredReaders: make([]uuid.UUID, 0),
		lendingMap:        make(map[uuid.UUID]uuid.UUID),
		readerBooksMap:    make(map[uuid.UUID][]uuid.UUID),
		lastRefresh:       time.Now(),
	}
}

// =================================================================
// FAST LOOKUP METHODS - Used by actors for decision making
// =================================================================

// GetAvailableBooks returns a list of book IDs that can be borrowed.
func (s *SimulationState) GetAvailableBooks() []uuid.UUID {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return copy to avoid race conditions.
	available := make([]uuid.UUID, len(s.availableBookIDs))
	copy(available, s.availableBookIDs)
	return available
}

// IsBookAvailable checks if a specific book can be borrowed.
func (s *SimulationState) IsBookAvailable(bookID uuid.UUID) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	book, exists := s.books[bookID]
	return exists && book.Available
}

// GetRegisteredReaders returns a list of active reader IDs.
func (s *SimulationState) GetRegisteredReaders() []uuid.UUID {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return copy to avoid race conditions.
	readers := make([]uuid.UUID, len(s.registeredReaders))
	copy(readers, s.registeredReaders)
	return readers
}

// GetReaderBorrowedBooks returns books currently borrowed by a specific reader.
func (s *SimulationState) GetReaderBorrowedBooks(readerID uuid.UUID) []uuid.UUID {
	s.mu.RLock()
	defer s.mu.RUnlock()

	reader, exists := s.readers[readerID]
	if !exists {
		return nil
	}

	// Return copy to avoid race conditions.
	books := make([]uuid.UUID, len(reader.BorrowedBooks))
	copy(books, reader.BorrowedBooks)
	return books
}

// GetReadersWithBorrowedBooks returns readers who have books to return.
func (s *SimulationState) GetReadersWithBorrowedBooks() []uuid.UUID {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var readersWithBooks []uuid.UUID
	for readerID, reader := range s.readers {
		if len(reader.BorrowedBooks) > 0 {
			readersWithBooks = append(readersWithBooks, readerID)
		}
	}

	return readersWithBooks
}

// GetStats returns current simulation statistics.
func (s *SimulationState) GetStats() SimulationStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.stats
}

// =================================================================
// STATE UPDATE METHODS - Called after successful operations
// =================================================================

// AddBook registers a new book in the system (available for borrowing).
func (s *SimulationState) AddBook(bookID uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	book := &BookState{
		ID:        bookID,
		Available: true,
		LentTo:    uuid.Nil,
		AddedAt:   time.Now(),
	}

	s.books[bookID] = book
	s.availableBookIDs = append(s.availableBookIDs, bookID)
	s.totalBooks++

	// Update stats.
	s.stats.TotalBooks = s.totalBooks
	s.stats.AvailableBooks = len(s.availableBookIDs)
	s.stats.BookAdditions++
}

// RemoveBook removes a book from the system (only if available).
func (s *SimulationState) RemoveBook(bookID uuid.UUID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	book, exists := s.books[bookID]
	if !exists || !book.Available {
		return false // Book doesn't exist or is currently lent.
	}

	// Remove from maps.
	delete(s.books, bookID)
	delete(s.lendingMap, bookID)

	// Remove from available books slice.
	s.availableBookIDs = removeFromSlice(s.availableBookIDs, bookID)
	s.totalBooks--

	// Update stats.
	s.stats.TotalBooks = s.totalBooks
	s.stats.AvailableBooks = len(s.availableBookIDs)
	s.stats.BookRemovals++

	return true
}

// RegisterReader adds a new reader to the system.
func (s *SimulationState) RegisterReader(readerID uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	reader := &ReaderState{
		ID:            readerID,
		Lifecycle:     Registered,
		BorrowedBooks: make([]uuid.UUID, 0, MaxBooksPerReader),
		LastActivity:  time.Now(),
	}

	s.readers[readerID] = reader
	s.registeredReaders = append(s.registeredReaders, readerID)
	s.totalReaders++

	// Update stats
	s.stats.TotalReaders = s.totalReaders
	s.stats.ReaderRegistrations++
}

// CancelReader removes a reader from the system (only if no borrowed books).
func (s *SimulationState) CancelReader(readerID uuid.UUID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	reader, exists := s.readers[readerID]
	if !exists || len(reader.BorrowedBooks) > 0 {
		return false // Reader doesn't exist or has borrowed books
	}

	// Remove from maps
	delete(s.readers, readerID)

	// Remove from registered readers slice
	s.registeredReaders = removeFromSlice(s.registeredReaders, readerID)
	s.totalReaders--

	// Update stats
	s.stats.TotalReaders = s.totalReaders
	s.stats.ReaderCancellations++

	return true
}

// LendBook records a book being lent to a reader.
func (s *SimulationState) LendBook(bookID, readerID uuid.UUID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if the book is available
	book, bookExists := s.books[bookID]
	if !bookExists || !book.Available {
		return false
	}

	// Check if the reader exists and has capacity
	reader, readerExists := s.readers[readerID]
	if !readerExists || len(reader.BorrowedBooks) >= MaxBooksPerReader {
		return false
	}

	// Update book state
	book.Available = false
	book.LentTo = readerID

	// Update reader state
	reader.BorrowedBooks = append(reader.BorrowedBooks, bookID)
	reader.LastActivity = time.Now()

	// Update lending map
	s.lendingMap[bookID] = readerID

	// Remove from available books
	s.availableBookIDs = removeFromSlice(s.availableBookIDs, bookID)
	s.totalActiveLendings++

	// Update stats
	s.stats.AvailableBooks = len(s.availableBookIDs)
	s.stats.ActiveLendings = s.totalActiveLendings

	return true
}

// ReturnBook records a book being returned by a reader.
func (s *SimulationState) ReturnBook(bookID, readerID uuid.UUID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if the book is lent to this reader
	book, bookExists := s.books[bookID]
	if !bookExists || book.Available || book.LentTo != readerID {
		return false
	}

	// Check if the reader has this book
	reader, readerExists := s.readers[readerID]
	if !readerExists {
		return false
	}

	// Remove book from reader's borrowed list
	reader.BorrowedBooks = removeFromSlice(reader.BorrowedBooks, bookID)
	reader.LastActivity = time.Now()

	// Update book state
	book.Available = true
	book.LentTo = uuid.Nil

	// Update maps
	delete(s.lendingMap, bookID)
	s.availableBookIDs = append(s.availableBookIDs, bookID)
	s.totalActiveLendings--

	// Update stats
	s.stats.AvailableBooks = len(s.availableBookIDs)
	s.stats.ActiveLendings = s.totalActiveLendings
	s.stats.CompletedLendings++

	return true
}

// =================================================================
// PERIODIC REFRESH - Sync with EventStore truth
// =================================================================

// ShouldRefresh checks if the state cache needs refreshing from the EventStore.
func (s *SimulationState) ShouldRefresh() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return time.Since(s.lastRefresh) > time.Duration(StateRefreshIntervalMs)*time.Millisecond &&
		!s.refreshInProgress
}

// RefreshFromEventStore updates the in-memory state from EventStore truth.
func (s *SimulationState) RefreshFromEventStore(ctx context.Context, handlers *HandlerBundle) error {
	s.mu.Lock()
	s.refreshInProgress = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.refreshInProgress = false
		s.lastRefresh = time.Now()
		s.mu.Unlock()
	}()

	// Query books in circulation with extended timeout
	booksResult, err := handlers.QueryBooksInCirculationForState(ctx)
	if err != nil {
		log.Printf("⚠️ State refresh failed (books query), keeping previous state: %v", err)
		return nil // Continue with stale state rather than breaking simulation
	}

	// Query registered readers with extended timeout
	readersResult, err := handlers.QueryRegisteredReadersForState(ctx)
	if err != nil {
		log.Printf("⚠️ State refresh failed (readers query), keeping previous state: %v", err)
		return nil // Continue with stale state rather than breaking simulation
	}

	// Query books lent out
	lentBooksResult, err := handlers.QueryBooksLentOut(ctx)
	if err != nil {
		log.Printf("⚠️ State refresh failed (lendings query), keeping previous state: %v", err)
		return nil // Continue with stale state rather than breaking simulation
	}

	// Rebuild state from query results (under lock)
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear existing state
	s.books = make(map[uuid.UUID]*BookState)
	s.readers = make(map[uuid.UUID]*ReaderState)
	s.lendingMap = make(map[uuid.UUID]uuid.UUID)
	s.readerBooksMap = make(map[uuid.UUID][]uuid.UUID)
	s.availableBookIDs = nil
	s.registeredReaders = nil

	// Rebuild books from circulation data
	s.rebuildBooksFromQuery(booksResult)

	// Rebuild readers from registration data
	s.rebuildReadersFromQuery(readersResult)

	// Rebuild lending relationships from lending data
	s.rebuildLendingFromQuery(lentBooksResult)

	// Update statistics
	s.totalBooks = len(s.books)
	s.totalReaders = len(s.readers)
	s.stats.TotalBooks = s.totalBooks
	s.stats.TotalReaders = s.totalReaders
	s.stats.AvailableBooks = len(s.availableBookIDs)
	s.stats.BooksLentOut = len(lentBooksResult.Lendings)
	s.stats.StateRefreshes++

	// DEBUG: Log book availability after state refresh
	if s.stats.StateRefreshes%50 == 0 { // Log every 50th refresh (every ~100 seconds)
		log.Printf("📚 State refresh #%d: %d total books, %d available, %d lent out",
			s.stats.StateRefreshes, s.totalBooks, len(s.availableBookIDs), len(lentBooksResult.Lendings))
	}

	return nil
}

// =================================================================
// HELPER FUNCTIONS
// =================================================================

// removeFromSlice removes the first occurrence of an item from a UUID slice.
func removeFromSlice(slice []uuid.UUID, item uuid.UUID) []uuid.UUID {
	for i, v := range slice {
		if v == item {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

// rebuildBooksFromQuery rebuilds book state from circulation query results.
func (s *SimulationState) rebuildBooksFromQuery(booksResult booksincirculation.BooksInCirculation) {
	for _, book := range booksResult.Books {
		bookID, err := uuid.Parse(book.BookID)
		if err != nil {
			continue // Skip invalid UUIDs
		}

		bookState := &BookState{
			ID:        bookID,
			Available: !book.IsCurrentlyLent,
			LentTo:    uuid.Nil,
			AddedAt:   book.AddedAt,
		}

		s.books[bookID] = bookState

		if bookState.Available {
			s.availableBookIDs = append(s.availableBookIDs, bookID)
		}
	}
}

// rebuildReadersFromQuery rebuilds reader state from registration query results.
func (s *SimulationState) rebuildReadersFromQuery(readersResult registeredreaders.RegisteredReaders) {
	for _, reader := range readersResult.Readers {
		readerID, err := uuid.Parse(reader.ReaderID)
		if err != nil {
			continue // Skip invalid UUIDs
		}

		readerState := &ReaderState{
			ID:           readerID,
			Name:         reader.Name,
			Lifecycle:    Registered,
			RegisteredAt: reader.RegisteredAt,
		}

		s.readers[readerID] = readerState
		s.registeredReaders = append(s.registeredReaders, readerID)
	}
}

// rebuildLendingFromQuery rebuilds lending relationships from lending query results.
func (s *SimulationState) rebuildLendingFromQuery(lentBooksResult bookslentout.BooksLentOut) {
	for _, lending := range lentBooksResult.Lendings {
		bookID, err := uuid.Parse(lending.BookID)
		if err != nil {
			continue // Skip invalid UUIDs
		}

		readerID, err := uuid.Parse(lending.ReaderID)
		if err != nil {
			continue // Skip invalid UUIDs
		}

		// Update book state to reflect lending
		if book, exists := s.books[bookID]; exists {
			book.Available = false
			book.LentTo = readerID

			// Remove from available books list (queries are now consistent, so this is redundant but harmless)
			s.availableBookIDs = removeFromSlice(s.availableBookIDs, bookID)
		}

		// Update lending maps
		s.lendingMap[bookID] = readerID

		if s.readerBooksMap[readerID] == nil {
			s.readerBooksMap[readerID] = make([]uuid.UUID, 0)
		}
		s.readerBooksMap[readerID] = append(s.readerBooksMap[readerID], bookID)

		// Also update the ReaderState.BorrowedBooks
		if reader, exists := s.readers[readerID]; exists {
			reader.BorrowedBooks = append(reader.BorrowedBooks, bookID)
		}
	}
}
