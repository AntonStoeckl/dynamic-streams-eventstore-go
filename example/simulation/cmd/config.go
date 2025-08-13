package main

import (
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"
)

const (
	defaultRate              = 70
	defaultMinBooks          = 60000 // München branch: ~62,500 books
	defaultMaxBooks          = 65000 // Allow some growth above minimum
	defaultMinReaders        = 14000 // München branch: ~14,000 active borrowers
	defaultMaxReaders        = 15000 // Allow some growth above minimum
	defaultSetupSleepSeconds = 60
)

// Config holds all simulation configuration parameters.
type Config struct {
	Rate                 int
	ObservabilityEnabled bool
	MinBooks             int
	MaxBooks             int
	MinReaders           int
	MaxReaders           int
	SetupSleepSeconds    int
	ErrorProbabilities   ErrorConfig
	CPUProfile           string
	MemProfile           string
}

// ErrorConfig holds probabilities for different error scenarios (as percentages 0-100).
type ErrorConfig struct {
	IdempotentRepeat           float64 // 0.5% - randomly repeat same operation
	LibraryManagerBookConflict float64 // 2.0% - try to remove just-lent book
	ReaderBorrowRemovedBook    float64 // 0.5% - try to borrow just-removed book
}

// parseFlags parses command line flags and returns configuration.
func parseFlags() Config {
	var (
		rate               = flag.Int("rate", defaultRate, "Requests per second")
		observability      = flag.Bool("observability-enabled", false, "Enable OpenTelemetry observability")
		minBooks           = flag.Int("min-books", defaultMinBooks, "Minimum number of books in circulation")
		maxBooks           = flag.Int("max-books", defaultMaxBooks, "Maximum number of books in circulation")
		minReaders         = flag.Int("min-readers", defaultMinReaders, "Minimum number of registered readers")
		maxReaders         = flag.Int("max-readers", defaultMaxReaders, "Maximum number of registered readers")
		setupSleepSeconds  = flag.Int("setup-sleep", defaultSetupSleepSeconds, "Sleep seconds between setup and main simulation")
		errorProbabilities = flag.String("error-rates", "0.5,2.0,0.5", "Comma-separated error probabilities: idempotent,manager-conflict,reader-removed-book")
		cpuProfile         = flag.String("cpuprofile", "", "write cpu profile to file")
		memProfile         = flag.String("memprofile", "", "write memory profile to file")
	)

	flag.Parse()

	// Parse error probabilities
	errorConfig, err := parseErrorProbabilities(*errorProbabilities)
	if err != nil {
		log.Fatalf("Invalid error probabilities '%s': %v", *errorProbabilities, err)
	}

	// Validate ranges
	if *minBooks >= *maxBooks {
		log.Fatalf("min-books (%d) must be less than max-books (%d)", *minBooks, *maxBooks)
	}

	if *minReaders >= *maxReaders {
		log.Fatalf("min-readers (%d) must be less than max-readers (%d)", *minReaders, *maxReaders)
	}

	return Config{
		Rate:                 *rate,
		ObservabilityEnabled: *observability,
		MinBooks:             *minBooks,
		MaxBooks:             *maxBooks,
		MinReaders:           *minReaders,
		MaxReaders:           *maxReaders,
		SetupSleepSeconds:    *setupSleepSeconds,
		ErrorProbabilities:   errorConfig,
		CPUProfile:           *cpuProfile,
		MemProfile:           *memProfile,
	}
}

// parseErrorProbabilities parses comma-separated error probability percentages.
func parseErrorProbabilities(probabilitiesStr string) (ErrorConfig, error) {
	parts := strings.Split(probabilitiesStr, ",")
	if len(parts) != 3 {
		return ErrorConfig{}, fmt.Errorf("expected 3 probabilities, got %d", len(parts))
	}

	probabilities := make([]float64, 3)
	for i, part := range parts {
		prob, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil {
			return ErrorConfig{}, fmt.Errorf("invalid probability '%s': %w", part, err)
		}
		if prob < 0 || prob > 100 {
			return ErrorConfig{}, fmt.Errorf("probability %f out of range [0, 100]", prob)
		}
		probabilities[i] = prob
	}

	return ErrorConfig{
		IdempotentRepeat:           probabilities[0],
		LibraryManagerBookConflict: probabilities[1],
		ReaderBorrowRemovedBook:    probabilities[2],
	}, nil
}
