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

	// InitialActiveReaders defines the conservative starting point for active readers.
	InitialActiveReaders = 150
	// MinActiveReaders defines the minimum scale limit.
	MinActiveReaders = 50
	// MaxActiveReaders defines the upper safety limit.
	MaxActiveReaders = 600

	// LibrarianCount defines the number of librarian staff (Acquisitions and Maintenance roles).
	LibrarianCount = 4

	// READER BEHAVIOR PATTERNS ...

	// MinBooksPerVisit defines the minimum books borrowed per visit.
	MinBooksPerVisit = 1
	// MaxBooksPerVisit defines the maximum books borrowed per visit.
	MaxBooksPerVisit = 5
	// MaxBooksPerReader defines the business rule limit per reader.
	MaxBooksPerReader = 10

	// ChanceReturnAll defines the probability that readers return all borrowed books.
	ChanceReturnAll = 0.8 // Natural behavior: 80% return all, 20% keep 1-2 books

	// ChanceBorrowAfterReturn defines the probability to browse/borrow books after returning.
	ChanceBorrowAfterReturn = 0.7 // Natural behavior: 70% browse after returning books

	// ChancePreferReadersWithBooks defines the probability to select readers with borrowed books during activation.
	// This creates a 50/50 balance between encouraging returns and discovering new patterns.
	ChancePreferReadersWithBooks = 0.5 // 50% prefer readers with books, 50% random selection

	// ChanceSyncOnActivation defines the probability to sync reader books when newly activated.
	// This provides realistic business behavior metrics without affecting the simulation state.
	ChanceSyncOnActivation = 0.1 // 10% chance to query BooksLentByReader for metrics

	// BROWSING AND DISCOVERY PATTERNS ...

	// ChanceBrowseOnline defines the probability that readers browse online catalog first.
	ChanceBrowseOnline = 0.1
	// ChanceVisitDirectly defines the probability that readers visit the library directly.
	ChanceVisitDirectly = 0.8

	// OnlineWishlistSize defines the maximum items in online wishlist.
	OnlineWishlistSize = 3

	// POPULATION DYNAMICS ...

	// ReaderCancellationRate defines the probability to cancel when above min readers.
	ReaderCancellationRate = 0.01

	// BookAdditionBatchSize defines the number of books added when below the minimum.
	BookAdditionBatchSize = 10
	// BookRemovalBatchSize defines the number of books removed when above the maximum.
	BookRemovalBatchSize = 5

	// LibrarianMaintenanceChance defines the probability to remove old books.
	LibrarianMaintenanceChance = 0.5

	// AUTO-TUNING SYSTEM ...

	// TargetP50LatencyMs defines the acceptable average response time in milliseconds.
	TargetP50LatencyMs = 80
	// TargetP99LatencyMs defines the maximum acceptable latency in milliseconds.
	TargetP99LatencyMs = 800
	// MaxTimeoutRate defines the timeout threshold as a percentage.
	MaxTimeoutRate = 0.005

	// AnomalyDetectionMultiplier defines the threshold for detecting metrics anomalies.
	// P99 is considered anomalous if it exceeds P50 * AnomalyDetectionMultiplier.
	AnomalyDetectionMultiplier = 80.0 // Was 10x, increased to reduce false alarms

	// ScaleUpIncrement defines the number of readers to add when performing well.
	ScaleUpIncrement = 10
	// ScaleDownIncrement defines the number of readers to remove when overloaded.
	ScaleDownIncrement = 20

	// TuningIntervalSeconds defines how often to adjust the active reader count.
	TuningIntervalSeconds = 5

	// BATCH PROCESSING CONFIGURATION ...

	// ActorBatchSize defines the number of actors processed per goroutine to avoid 14k goroutines.
	ActorBatchSize = 50
	// BatchProcessingDelayMs defines the delay between batch processing rounds.
	BatchProcessingDelayMs = 100

	// STATE MANAGEMENT ...

	// StateRefreshIntervalMs defines how often to refresh a cached state.
	// Increased to reduce database pressure with 60K books + 15K readers queries.
	StateRefreshIntervalMs = 2000
	// MetricsWindowSize defines the number of operations for metrics calculation.
	// Balanced at 500 ops for stable auto-tuning (~15-20s window at 30 ops/s).
	MetricsWindowSize = 500

	// SIMULATION TIMING ...

	// SetupPhaseDelaySeconds defines the initial setup time for visibility.
	SetupPhaseDelaySeconds = 5

	// TIMEOUT CONFIGURATION - Operation timeout durations ...

	// CommandTimeoutSeconds defines timeout for command operations (ExecuteLendBook, ExecuteReturnBook, etc.)
	// These are fast, transactional operations that should complete quickly.
	CommandTimeoutSeconds = 5.0

	// BooksInCirculationQueryTimeoutSeconds defines the timeout for this (relatively slow) query.
	BooksInCirculationQueryTimeoutSeconds = 60.0

	// BooksLentOutQueryTimeoutSeconds defines the timeout for this (relatively slow) query.
	BooksLentOutQueryTimeoutSeconds = 30.0

	// RegisteredReadersQueryTimeoutSeconds defines the timeout for this (relatively slow) query.
	RegisteredReadersQueryTimeoutSeconds = 5.0

	// BooksLentByReaderQueryTimeoutSeconds defines the timeout for this (fast) query.
	BooksLentByReaderQueryTimeoutSeconds = 0.5 // 500 ms

	// V1 SIMPLIFICATIONS - Advanced behavior patterns (not implemented in v1)!

	// Future reader behavior patterns (commented for v1).
	// ChanceKeepOneOrTwo         = 0.2   // 20% keep 1-2 unfinished books.
	// ChanceStayHome             = 0.4   // 40% stay home today.
	// ReaderRegistrationRate     = 0.7   // 70% chance to register when below max.
	// ThinkingTimeMs             = 1000  // Brief pause between major activities.

	// ERROR SCENARIOS (v2 - Not implemented in v1)!

	// Future error injection rates (commented for v1).
	// IdempotentRepeatRate           = 0.005 // 0.5% duplicate operations.
	// LibraryManagerConflictRate     = 0.02  // 2% manager conflicts.
	// ReaderBorrowRemovedBookRate    = 0.005 // 0.5% borrow removed books.
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
