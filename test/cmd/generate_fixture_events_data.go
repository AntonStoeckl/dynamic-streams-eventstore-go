package main

import (
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/test/userland/core"
)

const (
	NumSomethingHappenedEvents = 9000 // Number of "Something has happened" events to be created - adapt these as needed
	NumBookCopyEvents          = 1000 // Number of BookCopy events to be created - adapt these as needed

	OutputDir  = "test/fixtures" // the directory to put the fixture data into - should be fine as is
	OutputFile = "events.sql"    // the file to put the fixture data into - should be fine as is
)

type EventData struct {
	OccurredAt string
	EventType  string
	Payload    string
	Metadata   string
}

var metadataUUIDs []string

func main() {
	if err := GenerateFixtureDataSQL(); err != nil {
		panic(fmt.Sprintf("Error generating fixture data: %v\n", err))
	}
}

func GenerateFixtureDataSQL() error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to find project root: %w", err)
	}

	outputDir := filepath.Join(projectRoot, OutputDir)

	// Generate 100 UUIDs for metadata fields
	generateMetadataUUIDs()

	var events []EventData
	fakeClock := time.Unix(0, 0).UTC()

	// Generate "Something has happened" events
	events = append(events, generateSomethingHappenedEvents(NumSomethingHappenedEvents, &fakeClock)...)

	// Generate BookCopy events with realistic patterns
	events = append(events, generateBookCopyEvents(NumBookCopyEvents, &fakeClock)...)

	// Create an SQL INSERT statement and write to the file
	return writeSQLToFile(events, outputDir)
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

func generateSomethingHappenedEvents(numEvents int, fakeClock *time.Time) []EventData {
	var events []EventData
	eventPostfix := 0

	for totalEvents := 0; totalEvents < numEvents; {
		id, _ := uuid.NewV7()
		eventType := core.SomethingHasHappenedEventTypePrefix + strconv.Itoa(eventPostfix)

		*fakeClock = fakeClock.Add(time.Second)

		payload := fmt.Sprintf(`{"ID": "%s", "Description": "lorem ipsum dolor sit amet: %s", "occurredAt": "%s"}`,
			id.String(), id.String(), fakeClock.Format(time.RFC3339Nano))

		events = append(events, EventData{
			OccurredAt: fakeClock.Format(time.RFC3339Nano),
			EventType:  eventType,
			Payload:    payload,
			Metadata:   generateRandomMetadata(),
		})

		totalEvents++
		eventPostfix++
	}

	return events
}

func generateBookCopyEvents(numEvents int, fakeClock *time.Time) []EventData {
	var events []EventData
	booksInCirculation := make(map[uuid.UUID]bool)
	lentBooks := make(map[uuid.UUID]uuid.UUID) // bookID -> readerID
	removedBooks := make(map[uuid.UUID]bool)

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
	}

	eventsGenerated := 0

	for eventsGenerated < numEvents {
		bookID, _ := uuid.NewV7()
		book := books[rand.IntN(len(books))]

		// Always start with adding a book to circulation
		*fakeClock = fakeClock.Add(time.Second)
		payload := fmt.Sprintf(`{"BookID": "%s", "ISBN": "%s", "Title": "%s", "Author": "%s", "Edition": "%s", "Publisher": "%s", "Year": %d, "occurredAt": "%s"}`,
			bookID.String(), book.ISBN, book.Title, book.Author, book.Edition, book.Publisher, book.Year, fakeClock.Format(time.RFC3339Nano))

		events = append(events, EventData{
			OccurredAt: fakeClock.Format(time.RFC3339Nano),
			EventType:  core.BookCopyAddedToCirculationEventType,
			Payload:    payload,
			Metadata:   generateRandomMetadata(),
		})

		booksInCirculation[bookID] = true
		eventsGenerated++

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
				*fakeClock = fakeClock.Add(time.Duration(rand.IntN(86400)) * time.Second) // Random time up to 1 day
				lentPayload := fmt.Sprintf(`{"BookID": "%s", "ReaderID": "%s", "occurredAt": "%s"}`,
					bookID.String(), readerID.String(), fakeClock.Format(time.RFC3339Nano))

				events = append(events, EventData{
					OccurredAt: fakeClock.Format(time.RFC3339Nano),
					EventType:  core.BookCopyLentToReaderEventType,
					Payload:    lentPayload,
					Metadata:   generateRandomMetadata(),
				})
				lentBooks[bookID] = readerID
				eventsGenerated++

				if eventsGenerated >= numEvents {
					break
				}

				// Returned event (after some time)
				*fakeClock = fakeClock.Add(time.Duration(rand.IntN(2592000)) * time.Second) // Random time up to 30 days
				returnedPayload := fmt.Sprintf(`{"BookID": "%s", "ReaderID": "%s", "occurredAt": "%s"}`,
					bookID.String(), readerID.String(), fakeClock.Format(time.RFC3339Nano))

				events = append(events, EventData{
					OccurredAt: fakeClock.Format(time.RFC3339Nano),
					EventType:  core.BookCopyReturnedByReaderEventType,
					Payload:    returnedPayload,
					Metadata:   generateRandomMetadata(),
				})
				delete(lentBooks, bookID)
				eventsGenerated++
			}

		case action < 80: // 20% chance: lent but never returned
			if eventsGenerated < numEvents {
				readerID, _ := uuid.NewV7()

				*fakeClock = fakeClock.Add(time.Duration(rand.IntN(86400)) * time.Second)
				lentPayload := fmt.Sprintf(`{"BookID": "%s", "ReaderID": "%s", "occurredAt": "%s"}`,
					bookID.String(), readerID.String(), fakeClock.Format(time.RFC3339Nano))

				events = append(events, EventData{
					OccurredAt: fakeClock.Format(time.RFC3339Nano),
					EventType:  core.BookCopyLentToReaderEventType,
					Payload:    lentPayload,
					Metadata:   generateRandomMetadata(),
				})
				lentBooks[bookID] = readerID
				eventsGenerated++
			}

		default: // 20% chance: removed from circulation
			if eventsGenerated < numEvents {
				*fakeClock = fakeClock.Add(time.Duration(rand.IntN(86400)) * time.Second)
				removedPayload := fmt.Sprintf(`{"BookID": "%s", "occurredAt": "%s"}`,
					bookID.String(), fakeClock.Format(time.RFC3339Nano))

				events = append(events, EventData{
					OccurredAt: fakeClock.Format(time.RFC3339Nano),
					EventType:  core.BookCopyRemovedFromCirculationEventType,
					Payload:    removedPayload,
					Metadata:   generateRandomMetadata(),
				})
				removedBooks[bookID] = true
				delete(booksInCirculation, bookID)
				delete(lentBooks, bookID)
				eventsGenerated++
			}
		}
	}

	return events
}

func buildSQLInsert(events []EventData) string {
	if len(events) == 0 {
		return ""
	}

	var builder strings.Builder
	// just a little hack to avoid SQL inspection in the IDE
	builder.WriteString("INSERT " + "INTO " + "events (occurred_at, event_type, payload, metadata) VALUES\n")

	for i, event := range events {
		if i > 0 {
			builder.WriteString(",\n")
		}
		builder.WriteString(fmt.Sprintf("    ('%s', '%s', '%s', '%s')",
			event.OccurredAt,
			event.EventType,
			strings.ReplaceAll(event.Payload, "'", "''"),   // Escape single quotes
			strings.ReplaceAll(event.Metadata, "'", "''"))) // Escape single quotes in metadata too
	}

	builder.WriteString(";\n")

	return builder.String()
}

func writeSQLToFile(events []EventData, outputDir string) error {
	// Create the output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Build the full file path
	filePath := filepath.Join(outputDir, OutputFile)

	// Generate SQL content
	sqlContent := buildSQLInsert(events)

	// Write to the file
	if err := os.WriteFile(filePath, []byte(sqlContent), 0644); err != nil {
		return fmt.Errorf("failed to write SQL file: %w", err)
	}

	fmt.Printf("Successfully generated %d events and wrote SQL to %s\n", len(events), filePath)

	return nil
}
