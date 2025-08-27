package main

// tuning.go - All configurable parameters for the actor-based simulation.
// Centralized constants for easy experimentation and tuning.

const (
	// POPULATION PARAMETERS - München Library Branch Scale ...

	// MinReaders defines the minimum active borrowers to maintain based on München Stadtbibliothek research.
	MinReaders = 14000

	// MaxReaders defines the maximum readers before stopping registrations.
	MaxReaders = 15000

	// MinBooks defines the minimum books in circulation per branch.
	MinBooks = 60000

	// MaxBooks defines the maximum books before removal.
	MaxBooks = 65000

	// ACTOR POOL CONFIGURATION ...

	// DefaultActiveReaders defines the number of active readers for the simulation.
	// This is configurable via the command line.
	DefaultActiveReaders = 100

	// DefaultLibrarianCount defines the default number of librarian staff (Acquisitions and Maintenance roles).
	DefaultLibrarianCount = 8

	// READER BEHAVIOR PATTERNS ...

	// MinBooksPerVisit defines the minimum books borrowed per visit.
	MinBooksPerVisit = 1

	// MaxBooksPerVisit defines the maximum books borrowed per visit.
	MaxBooksPerVisit = 5

	// MaxBooksPerReader defines the business rule limit per reader.
	MaxBooksPerReader = 10

	// ChanceReturnAll defines the probability that readers return all borrowed books.
	ChanceReturnAll = 0.85 // Natural behavior: 85% return all, 15% keep 1-2 books

	// ChanceBorrowAfterReturn defines the probability to browse/borrow books after returning.
	ChanceBorrowAfterReturn = 0.65 // Natural behavior: 65% browse after returning books

	// ChancePreferReadersWithBooks defines the probability to select readers with borrowed books during activation.
	// This creates a balance between encouraging returns and discovering new patterns.
	ChancePreferReadersWithBooks = 0.7 // 70% prefer readers with books, 30% random selection

	// ChanceSyncOnActivation defines the probability to sync reader books when newly activated.
	// This provides realistic business behavior metrics without affecting the simulation state.
	ChanceSyncOnActivation = 0.1 // 10% chance to query BooksLentByReader for metrics

	// ChanceVisitDirectly defines the probability that readers visit the library directly.
	ChanceVisitDirectly = 0.8

	// DYNAMIC READER POPULATION SYSTEM ...

	// BaseRegisterRate defines the maximum registration probability at minimum readers.
	BaseRegisterRate = 0.30

	// BaseCancelRate defines the maximum cancellation probability at maximum readers.
	BaseCancelRate = 0.24

	// ReaderRandomness defines the strength of the random walk for reader population.
	ReaderRandomness = 0.06

	// DYNAMIC BOOKS MANAGEMENT SYSTEM ...

	// LibrarianWorkProbability defines the base probability for librarians to work.
	LibrarianWorkProbability = 0.4

	// BaseRandomness defines the strength of a random walk to prevent equilibrium.
	BaseRandomness = 0.3

	// WaveAmplitude defines the strength of sine wave patterns for natural fluctuations.
	WaveAmplitude = 0.2

	// BurstChance defines the probability of burst mode for occasional extremes.
	BurstChance = 0.02

	// MomentumDecay defines how quickly librarian momentum fades (0.0 to 1.0).
	MomentumDecay = 0.8

	// MomentumInfluence defines how much momentum affects probabilities.
	MomentumInfluence = 0.3

	// AcquisitionBurstSize defines the number of books added during acquisition bursts.
	AcquisitionBurstSize = 20

	// ClearanceBurstSize defines the number of books removed during clearance bursts.
	ClearanceBurstSize = 15

	// PERFORMANCE MEASUREMENT ...

	// RollingWindowSize defines the number of recent batches used for rolling average calculations.
	RollingWindowSize = 100

	// IDEMPOTENCY TESTING ...

	// IdempotentOperationProbability defines the chance to repeat a successful operation
	// for testing idempotency handling in the EventStore.
	IdempotentOperationProbability = 0.003 // 0.3% chance to repeat successful operations

	// BATCH PROCESSING CONFIGURATION ...

	// DefaultConcurrentWorkers defines the number of worker goroutines processing readers in parallel.
	// This limits concurrent requests to the EventStore.
	DefaultConcurrentWorkers = 3

	// SIMULATION TIMING ...

	// SetupPhaseDelaySeconds defines the initial setup time for visibility.
	SetupPhaseDelaySeconds = 5

	// TIMEOUT CONFIGURATION - Operation timeout durations ...

	// CommandTimeoutSeconds defines timeout for command operations (ExecuteLendBook, ExecuteReturnBook, etc.)
	// These are fast, transactional operations that should complete quickly.
	CommandTimeoutSeconds = 5.0

	// BatchTimeoutSeconds defines timeout for entire batch processing cycles.
	BatchTimeoutSeconds = 120.0

	// BooksInCirculationQueryTimeoutSeconds defines the timeout for this (relatively slow) query.
	BooksInCirculationQueryTimeoutSeconds = 180.0

	// BooksLentOutQueryTimeoutSeconds defines the timeout for this (relatively slow) query.
	BooksLentOutQueryTimeoutSeconds = 180.0

	// RegisteredReadersQueryTimeoutSeconds defines the timeout for this (relatively slow) query.
	RegisteredReadersQueryTimeoutSeconds = 20.0

	// BooksLentByReaderQueryTimeoutSeconds defines the timeout for this (fast) query.
	BooksLentByReaderQueryTimeoutSeconds = 2.0
)

// ReaderPersona represents different types of library users (future enhancement).
type ReaderPersona int

const (
	CasualReader ReaderPersona = iota // Occasional visits, 1-2 books.
	PowerUser                         // Frequent visits, multiple books.
	// Student                        // Research patterns, longer loans (future enhancement).
	// Future personas for realistic modeling.
)

// LibrarianRole defines different librarian responsibilities.
type LibrarianRole int

const (
	Acquisitions LibrarianRole = iota // Adds new books to the collection.
	Maintenance                       // Removes old/damaged books.
)
