package main

import (
	"sync"

	"github.com/google/uuid"
)

// SimulationState manages the current state of the library system for realistic scenario generation.
// All state updates are protected by mutex for concurrent access from worker pool.
type SimulationState struct {
	mu sync.RWMutex

	// books tracks books currently in circulation (BookID -> true)
	books map[string]bool

	// readers tracks registered (non-canceled) readers (ReaderID -> true)
	readers map[string]bool

	// lending tracks current book-to-reader lending relationships (BookID -> ReaderID)
	lending map[string]string

	// Stats for monitoring
	totalBooks        int
	totalReaders      int
	totalLending      int // Current active lendings
	completedLendings int // Total completed (returned) lendings
}

// NewSimulationState creates a new empty simulation state.
func NewSimulationState() *SimulationState {
	return &SimulationState{
		books:   make(map[string]bool),
		readers: make(map[string]bool),
		lending: make(map[string]string),
	}
}

// AddBook marks a book as being in circulation.
func (s *SimulationState) AddBook(bookID uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	bookIDStr := bookID.String()
	s.books[bookIDStr] = true
	s.totalBooks = len(s.books)
}

// RemoveBook removes a book from circulation and any associated lending.
func (s *SimulationState) RemoveBook(bookID uuid.UUID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	bookIDStr := bookID.String()
	if !s.books[bookIDStr] {
		return false // Book not in circulation
	}

	delete(s.books, bookIDStr)
	delete(s.lending, bookIDStr) // Remove any lending relationship
	s.totalBooks = len(s.books)
	s.totalLending = len(s.lending)

	return true
}

// AddReader marks a reader as registered.
func (s *SimulationState) AddReader(readerID uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	readerIDStr := readerID.String()
	s.readers[readerIDStr] = true
	s.totalReaders = len(s.readers)
}

// RemoveReader removes a reader from registered state and any associated lending.
func (s *SimulationState) RemoveReader(readerID uuid.UUID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	readerIDStr := readerID.String()
	if !s.readers[readerIDStr] {
		return false // Reader not registered
	}

	delete(s.readers, readerIDStr)

	// Remove any lending relationships for this reader
	for bookID, lentToReader := range s.lending {
		if lentToReader == readerIDStr {
			delete(s.lending, bookID)
		}
	}

	s.totalReaders = len(s.readers)
	s.totalLending = len(s.lending)

	return true
}

// LendBook creates a lending relationship between a book and reader.
func (s *SimulationState) LendBook(bookID, readerID uuid.UUID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	bookIDStr := bookID.String()
	readerIDStr := readerID.String()

	// Check if book is in circulation and not already lent
	if !s.books[bookIDStr] {
		return false // Book not in circulation
	}

	if _, isLent := s.lending[bookIDStr]; isLent {
		return false // Book already lent
	}

	// Check if reader is registered
	if !s.readers[readerIDStr] {
		return false // Reader not registered
	}

	s.lending[bookIDStr] = readerIDStr
	s.totalLending = len(s.lending)

	return true
}

// ReturnBook removes the lending relationship for a book.
func (s *SimulationState) ReturnBook(bookID, readerID uuid.UUID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	bookIDStr := bookID.String()
	readerIDStr := readerID.String()

	// Check if book is lent to this specific reader
	if lentToReader, exists := s.lending[bookIDStr]; !exists || lentToReader != readerIDStr {
		return false // Book not lent to this reader
	}

	delete(s.lending, bookIDStr)
	s.totalLending = len(s.lending)
	s.completedLendings++ // Track completed lending

	return true
}

// GetStats returns current state statistics (read-only).
func (s *SimulationState) GetStats() (books, readers, lending int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.totalBooks, s.totalReaders, s.totalLending
}

// GetDetailedStats returns detailed state statistics including completed lendings.
func (s *SimulationState) GetDetailedStats() (books, readers, activeLendings, completedLendings int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.totalBooks, s.totalReaders, s.totalLending, s.completedLendings
}

// GetAvailableBooks returns a slice of book IDs that are in circulation but not currently lent.
func (s *SimulationState) GetAvailableBooks() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var available []string
	for bookID := range s.books {
		if _, isLent := s.lending[bookID]; !isLent {
			available = append(available, bookID)
		}
	}

	return available
}

// GetLentBooks returns a slice of book IDs that are currently lent out.
func (s *SimulationState) GetLentBooks() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var lent []string
	for bookID := range s.lending {
		lent = append(lent, bookID)
	}

	return lent
}

// GetRegisteredReaders returns a slice of reader IDs that are currently registered.
func (s *SimulationState) GetRegisteredReaders() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var registered []string
	for readerID := range s.readers {
		registered = append(registered, readerID)
	}

	return registered
}

// IsBookInCirculation checks if a book is currently in circulation.
func (s *SimulationState) IsBookInCirculation(bookID uuid.UUID) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.books[bookID.String()]
}

// IsReaderRegistered checks if a reader is currently registered.
func (s *SimulationState) IsReaderRegistered(readerID uuid.UUID) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.readers[readerID.String()]
}

// IsBookLent checks if a book is currently lent out.
func (s *SimulationState) IsBookLent(bookID uuid.UUID) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, isLent := s.lending[bookID.String()]
	return isLent
}
