package main

import (
	"encoding/csv"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	core2 "github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

const (
	tenThousand     = 10000
	hundredThousand = tenThousand * 10
	million         = hundredThousand * 10

	// NumSomethingHappenedEvents - Number of "Something has happened" events to be created - adapt these as needed
	//
	// WARNING
	//
	// 10 Million fixture events in total create 3.9GB (CSV and SQL each) files which are mounted into a Docker volume.
	// The generation itself takes about 25 seconds.
	// The import takes about 4 minutes if done from the CSV in the DB with dropped indexes.
	// Importing this via SQL already takes ages.
	// If you go to 100 Million events, you might be in trouble ...
	NumSomethingHappenedEvents = 9 * million

	// NumBookCopyEvents - Number of "BookCopy..." events to be created - adapt these as needed
	//
	// WARNING
	//
	// 10 Million fixture events in total create 3.9GB (CSV and SQL each) files which are mounted into a Docker volume.
	// The generation itself takes about 25 seconds.
	// The import takes about 4 minutes if done from the CSV in the DB with dropped indexes.
	// Importing this via SQL already takes ages.
	// If you go to 100 Million events, you might be in trouble ...
	NumBookCopyEvents = 1 * million

	// WriteCSVFileEnabled determines whether the fixtures are written to a CSV file (recommended, see above).
	WriteCSVFileEnabled = true

	// WriteSQLFileEnabled determines whether the fixtures are written to a SQL file (not recommended, see above).
	WriteSQLFileEnabled = true

	// OutputDir - the directory to put the fixture data into - don't change.
	OutputDir = "testutil/postgresengine/fixtures"

	// OutputSQLFile - the SQL file to put the fixture data into - don't change.
	OutputSQLFile = "events.sql"

	// OutputCSVFile - the CSV file to put the fixture data into - don't change.
	OutputCSVFile = "events.csv"
)

type EventData struct {
	OccurredAt string
	EventType  string
	Payload    string
	Metadata   string
}

type Writers struct {
	sqlFile    *os.File
	csvFile    *os.File
	csvWriter  *csv.Writer
	eventCount int
}

var metadataUUIDs []string

func main() {
	if err := GenerateFixtureDataSQL(); err != nil {
		panic(fmt.Sprintf("Error generating fixture data: %v\n", err))
	}
}

func GenerateFixtureDataSQL() error {
	startTime := time.Now()
	totalEvents := NumSomethingHappenedEvents + NumBookCopyEvents

	// Report what we're starting
	fmt.Println("🚀 Starting fixture data generation")
	fmt.Printf("📊 Total events to generate: %s (%s SomethingHappened + %s BookCopy)\n",
		formatNumber(totalEvents), formatNumber(NumSomethingHappenedEvents), formatNumber(NumBookCopyEvents))

	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to find project root: %w", err)
	}

	outputDir := filepath.Join(projectRoot, OutputDir)

	// Create the output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Report output formats
	fmt.Println("📁 Output formats:")
	if WriteCSVFileEnabled {
		fmt.Printf("  ✅ CSV: %s\n", filepath.Join(outputDir, OutputCSVFile))
	} else {
		fmt.Println("  ✖️ CSV: disabled")
	}
	if WriteSQLFileEnabled {
		fmt.Printf("  ✅ SQL: %s\n", filepath.Join(outputDir, OutputSQLFile))
	} else {
		fmt.Println("  ✖️ SQL: disabled")
	}
	fmt.Println()

	// Generate 100 UUIDs for metadata fields
	generateMetadataUUIDs()

	writers, err := setupWriters(outputDir)
	if err != nil {
		return err
	}
	defer closeWriters(writers)

	fakeClock := time.Unix(0, 0).UTC()

	// Generate and write "Something has happened" events
	fmt.Printf("🔄 Phase 1/2: Generating %s SomethingHappened events...\n", formatNumber(NumSomethingHappenedEvents))
	err = generateSomethingHappenedEvents(writers, NumSomethingHappenedEvents, &fakeClock)
	if err != nil {
		return err
	}
	fmt.Printf("\n✅ Phase 1/2: Generated %s SomethingHappened events\n\n", formatNumber(NumSomethingHappenedEvents))

	// Generate and write BookCopy events
	fmt.Printf("🔄 Phase 2/2: Generating %s BookCopy events...\n", formatNumber(NumBookCopyEvents))
	err = generateBookCopyEvents(writers, NumBookCopyEvents, &fakeClock)
	if err != nil {
		return err
	}
	fmt.Printf("\n✅ Phase 2/2: Generated %s BookCopy events\n\n", formatNumber(NumBookCopyEvents))

	// Finalize SQL file
	if WriteSQLFileEnabled {
		_, err = writers.sqlFile.WriteString(";\n")
		if err != nil {
			return fmt.Errorf("failed to write SQL footer: %w", err)
		}
	}

	elapsed := time.Since(startTime)

	fmt.Printf("Fixture generation completed! 🎉\n")
	fmt.Printf("Total events generated: %s 📊\n", formatNumber(totalEvents))
	fmt.Printf("Total time: %v ⏱️\n", elapsed.Round(time.Millisecond))
	if WriteCSVFileEnabled {
		fmt.Printf("CSV file: %s\n", filepath.Join(outputDir, OutputCSVFile))
	}
	if WriteSQLFileEnabled {
		fmt.Printf("SQL file: %s\n", filepath.Join(outputDir, OutputSQLFile))
	}

	return nil
}

func setupWriters(outputDir string) (*Writers, error) {
	writers := &Writers{}

	if WriteSQLFileEnabled {
		sqlPath := filepath.Join(outputDir, OutputSQLFile)
		sqlFile, err := os.Create(sqlPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create SQL file: %w", err)
		}
		writers.sqlFile = sqlFile

		// Write SQL header
		_, err = sqlFile.WriteString("INSERT " + "INTO " + "events (occurred_at, event_type, payload, metadata) VALUES\n")
		if err != nil {
			return nil, fmt.Errorf("failed to write SQL header: %w", err)
		}
	}

	if WriteCSVFileEnabled {
		csvPath := filepath.Join(outputDir, OutputCSVFile)
		csvFile, err := os.Create(csvPath)
		if err != nil {
			if writers.sqlFile != nil {
				_ = writers.sqlFile.Close() // makes no sense to handle this
			}
			return nil, fmt.Errorf("failed to create CSV file: %w", err)
		}

		writers.csvFile = csvFile
		writers.csvWriter = csv.NewWriter(csvFile)
	}

	return writers, nil
}

func closeWriters(writers *Writers) {
	if writers.csvWriter != nil {
		writers.csvWriter.Flush()
	}
	if writers.sqlFile != nil {
		closeErr := writers.sqlFile.Close()
		if closeErr != nil {
			// Handle close error if needed
		}
	}
}

func writeEvent(writers *Writers, event EventData) error {
	if WriteSQLFileEnabled {
		separator := ""
		if writers.eventCount > 0 {
			separator = ",\n"
		}

		sqlValue := fmt.Sprintf(`%s    ('%s', '%s', '%s', '%s')`,
			separator,
			event.OccurredAt,
			event.EventType,
			strings.ReplaceAll(event.Payload, "'", "''"),
			strings.ReplaceAll(event.Metadata, "'", "''"))

		_, err := writers.sqlFile.WriteString(sqlValue)
		if err != nil {
			return fmt.Errorf("failed to write SQL value: %w", err)
		}
	}

	if WriteCSVFileEnabled {
		record := []string{
			event.OccurredAt,
			event.EventType,
			event.Payload,
			event.Metadata,
		}

		if err := writers.csvWriter.Write(record); err != nil {
			return fmt.Errorf("failed to write CSV record: %w", err)
		}
	}

	writers.eventCount++
	return nil
}

func generateMetadataUUIDs() {
	metadataUUIDs = make([]string, 100)
	for i := 0; i < 100; i++ {
		uid, _ := uuid.NewV7()
		metadataUUIDs[i] = uid.String()
	}
}

func generateRandomMetadata() string {
	messageID := metadataUUIDs[rand.IntN(len(metadataUUIDs))]
	causationID := metadataUUIDs[rand.IntN(len(metadataUUIDs))]
	correlationID := metadataUUIDs[rand.IntN(len(metadataUUIDs))]

	return fmt.Sprintf(`{"MessageID": "%s", "CausationID": "%s", "CorrelationID": "%s"}`,
		messageID, causationID, correlationID)
}

func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Walk up the directory tree looking for go.mod
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached the root directory
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("could not find project root (no go.mod found)")
}

func formatNumber(n int) string {
	if n >= million {
		return fmt.Sprintf("%.1fM", float64(n)/float64(million))
	} else if n >= hundredThousand {
		return fmt.Sprintf("%.0fK", float64(n)/1000)
	} else if n >= tenThousand {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return strconv.Itoa(n)
}

func reportProgress(
	phase string,
	current int,
	total int,
) {

	percentage := float64(current) / float64(total) * 100
	if int(percentage)%5 == 0 && current > 0 {
		var spinChar string
		if percentage >= 100 {
			spinChar = "●" // Filled circle when complete
		} else {
			spinner := []string{"◐", "◓", "◑", "◒"}
			spinChar = spinner[int(percentage/5)%len(spinner)]
		}
		fmt.Printf("\r  %s %s: %s/%s (%.0f%%)    ", spinChar, phase, formatNumber(current), formatNumber(total), percentage)
	}
}

func generateSomethingHappenedEvents(writers *Writers, numEvents int, fakeClock *time.Time) error {
	eventPostfix := 0
	progressStep := numEvents / 20 // 5% increments

	for totalGenerated := 0; totalGenerated < numEvents; totalGenerated++ {
		id, _ := uuid.NewV7()
		eventType := core2.SomethingHasHappenedEventTypePrefix + strconv.Itoa(eventPostfix)

		*fakeClock = fakeClock.Add(time.Millisecond * 2)

		payload := fmt.Sprintf(`{"ID": "%s", "Description": "lorem ipsum dolor sit amet: %s", "occurredAt": "%s"}`,
			id.String(), id.String(), fakeClock.Format(time.RFC3339Nano))

		metadata := generateRandomMetadata()

		event := EventData{
			OccurredAt: fakeClock.Format(time.RFC3339Nano),
			EventType:  eventType,
			Payload:    payload,
			Metadata:   metadata,
		}

		if err := writeEvent(writers, event); err != nil {
			return err
		}

		eventPostfix++

		// Report progress every 5%
		if progressStep > 0 && (totalGenerated+1)%progressStep == 0 {
			reportProgress("SomethingHappened", totalGenerated+1, numEvents)
		}
	}

	return nil
}

func generateBookCopyEvents(writers *Writers, numEvents int, fakeClock *time.Time) error {
	booksInCirculation := make(map[uuid.UUID]bool)
	lentBooks := make(map[uuid.UUID]uuid.UUID) // bookID -> readerID
	removedBooks := make(map[uuid.UUID]bool)
	progressStep := numEvents / 20 // 5% increments

	// Sample book data
	books := []struct {
		ISBN      string
		Title     string
		Author    string
		Edition   string
		Publisher string
		Year      int
	}{
		{"978-1-098-10013-1", "Learning Domain-Driven Design", "Vlad Khononov", "First Edition", "O'Reilly Media, Inc.", 2021},
		{"978-0-321-12521-7", "Domain-Driven Design", "Eric Evans", "First Edition", "Addison-Wesley", 2003},
		{"978-1-617-29428-6", "Microservices Patterns", "Chris Richardson", "First Edition", "Manning Publications", 2018},
		{"978-1-449-37320-0", "Building Microservices", "Sam Newman", "First Edition", "O'Reilly Media", 2015},
		{"978-0-596-00696-5", "Head First Design Patterns", "Eric Freeman", "First Edition", "O'Reilly Media", 2004},
	}

	eventsGenerated := 0

	for eventsGenerated < numEvents {
		bookID, _ := uuid.NewV7()
		book := books[rand.IntN(len(books))]

		// Always start with adding a book to circulation
		*fakeClock = fakeClock.Add(time.Millisecond * 2)
		payload := fmt.Sprintf(`{"BookID": "%s", "ISBN": "%s", "Title": "%s", "Author": "%s", "Edition": "%s", "Publisher": "%s", "Year": %d, "occurredAt": "%s"}`,
			bookID.String(), book.ISBN, book.Title, book.Author, book.Edition, book.Publisher, book.Year, fakeClock.Format(time.RFC3339Nano))

		metadata := generateRandomMetadata()

		event := EventData{
			OccurredAt: fakeClock.Format(time.RFC3339Nano),
			EventType:  core2.BookCopyAddedToCirculationEventType,
			Payload:    payload,
			Metadata:   metadata,
		}

		if err := writeEvent(writers, event); err != nil {
			return err
		}

		booksInCirculation[bookID] = true
		eventsGenerated++

		// Report progress every 5%
		if progressStep > 0 && eventsGenerated%progressStep == 0 {
			reportProgress("BookCopy", eventsGenerated, numEvents)
		}

		if eventsGenerated >= numEvents {
			break
		}

		// Decide what happens to this book (weighted probabilities)
		action := rand.IntN(100)

		switch {
		case action < 60: // 60% chance: multiple lent/returned cycles
			cycles := rand.IntN(3) + 1 // 1-3 cycles
			for i := 0; i < cycles && eventsGenerated < numEvents; i++ {
				readerID, _ := uuid.NewV7()

				// Lent event
				*fakeClock = fakeClock.Add(time.Duration(rand.IntN(86400)) * time.Millisecond) // Random time up to 1 day
				lentPayload := fmt.Sprintf(`{"BookID": "%s", "ReaderID": "%s", "occurredAt": "%s"}`,
					bookID.String(), readerID.String(), fakeClock.Format(time.RFC3339Nano))

				lentMetadata := generateRandomMetadata()

				lentEvent := EventData{
					OccurredAt: fakeClock.Format(time.RFC3339Nano),
					EventType:  core2.BookCopyLentToReaderEventType,
					Payload:    lentPayload,
					Metadata:   lentMetadata,
				}

				if err := writeEvent(writers, lentEvent); err != nil {
					return err
				}

				lentBooks[bookID] = readerID
				eventsGenerated++

				// Report progress every 5%
				if progressStep > 0 && eventsGenerated%progressStep == 0 {
					reportProgress("BookCopy", eventsGenerated, numEvents)
				}

				if eventsGenerated >= numEvents {
					break
				}

				// Returned event (after some time)
				*fakeClock = fakeClock.Add(time.Duration(rand.IntN(2592000)) * time.Millisecond) // Random time up to 30 days
				returnedPayload := fmt.Sprintf(`{"BookID": "%s", "ReaderID": "%s", "occurredAt": "%s"}`,
					bookID.String(), readerID.String(), fakeClock.Format(time.RFC3339Nano))

				returnedMetadata := generateRandomMetadata()

				returnedEvent := EventData{
					OccurredAt: fakeClock.Format(time.RFC3339Nano),
					EventType:  core2.BookCopyReturnedByReaderEventType,
					Payload:    returnedPayload,
					Metadata:   returnedMetadata,
				}

				if err := writeEvent(writers, returnedEvent); err != nil {
					return err
				}

				delete(lentBooks, bookID)
				eventsGenerated++

				// Report progress every 5%
				if progressStep > 0 && eventsGenerated%progressStep == 0 {
					reportProgress("BookCopy", eventsGenerated, numEvents)
				}
			}

		case action < 80: // 20% chance: lent but never returned
			if eventsGenerated < numEvents {
				readerID, _ := uuid.NewV7()

				*fakeClock = fakeClock.Add(time.Duration(rand.IntN(86400)) * time.Millisecond)
				lentPayload := fmt.Sprintf(`{"BookID": "%s", "ReaderID": "%s", "occurredAt": "%s"}`,
					bookID.String(), readerID.String(), fakeClock.Format(time.RFC3339Nano))

				lentMetadata := generateRandomMetadata()

				lentEvent := EventData{
					OccurredAt: fakeClock.Format(time.RFC3339Nano),
					EventType:  core2.BookCopyLentToReaderEventType,
					Payload:    lentPayload,
					Metadata:   lentMetadata,
				}

				if err := writeEvent(writers, lentEvent); err != nil {
					return err
				}

				lentBooks[bookID] = readerID
				eventsGenerated++

				// Report progress every 5%
				if progressStep > 0 && eventsGenerated%progressStep == 0 {
					reportProgress("BookCopy", eventsGenerated, numEvents)
				}
			}

		default: // 20% chance: removed from circulation
			if eventsGenerated < numEvents {
				*fakeClock = fakeClock.Add(time.Duration(rand.IntN(86400)) * time.Millisecond)
				removedPayload := fmt.Sprintf(`{"BookID": "%s", "occurredAt": "%s"}`,
					bookID.String(), fakeClock.Format(time.RFC3339Nano))

				removedMetadata := generateRandomMetadata()

				removedEvent := EventData{
					OccurredAt: fakeClock.Format(time.RFC3339Nano),
					EventType:  core2.BookCopyRemovedFromCirculationEventType,
					Payload:    removedPayload,
					Metadata:   removedMetadata,
				}

				if err := writeEvent(writers, removedEvent); err != nil {
					return err
				}

				removedBooks[bookID] = true
				delete(booksInCirculation, bookID)
				delete(lentBooks, bookID)
				eventsGenerated++

				// Report progress every 5%
				if progressStep > 0 && eventsGenerated%progressStep == 0 {
					reportProgress("BookCopy", eventsGenerated, numEvents)
				}
			}
		}
	}

	return nil
}
