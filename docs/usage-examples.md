# Usage Examples

Practical examples demonstrating entity-independent operations with dynamic-streams-eventstore-go.

**Note:** The `example/` directory contains working test fixtures and usage examples used by the test suite.

## Example: Library Book Lending System

This example demonstrates operations where a single business transaction affects multiple (logical) entities.

### Domain Events

```go
// BookCirculation entity events
type BookCopyAddedToCirculation struct {
    BookID     string   
    Title      string   
    OccurredAt time.Time
}

type BookCopyRemovedFromCirculation struct {
    BookID     string   
    Reason     string   
    OccurredAt time.Time
}

// ReaderAccount entity events
type ReaderRegistered struct {
    ReaderID   string   
    Name       string   
    OccurredAt time.Time
}

type ReaderContractCanceled struct {
    ReaderID   string   
    Reason     string   
    OccurredAt time.Time
}

// BookLending entity events
type BookCopyLentToReader struct {
    BookID     string   
    ReaderID   string   
    OccurredAt time.Time
}

type BookCopyReturnedByReader struct {
    BookID     string   
    ReaderID   string   
    OccurredAt time.Time
}

// Each event implements EventType() and HasOccurredAt() methods
```

### Core Use Case: Lending a Book

```go
import (
    "context"
    "encoding/json"
    "errors"
    "time"
    
    . "github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
)

func LendBookToReader(ctx context.Context, es EventStore, bookID, readerID string) error {
    // 1. Query entity-independent events  
    filter := BuildEventFilter().
        Matching().
        AnyEventTypeOf(
            "BookCopyAddedToCirculation", "BookCopyRemovedFromCirculation",
            "ReaderRegistered", "ReaderContractCanceled",
            "BookCopyLentToReader", "BookCopyReturnedByReader").
        AndAnyPredicateOf(P("BookID", bookID), P("ReaderID", readerID)).
        Finalize()
    
    events, maxSeq, err := es.Query(ctx, filter)
    if err != nil {
        return err
    }
    
    // 2. Build the current state from events
    bookAvailable, readerBorrowCount := buildStateFromEvents(events, bookID, readerID)
    // ... implementation details of buildStateFromEvents omitted for brevity ...
    
    // 3. Apply business rules
    if !bookAvailable {
        return errors.New("book not available")
    }
    if readerBorrowCount >= 5 { // max quota
        return errors.New("reader quota exceeded")
    }
    
    // 4. Create and append new event atomically
    payloadJSON, err := json.Marshal(map[string]string{"BookID": bookID, "ReaderID": readerID})
    if err != nil {
        return err
    }
    
    lendEvent, err := BuildStorableEventWithEmptyMetadata("BookCopyLentToReader", time.Now(), payloadJSON)
    if err != nil {
        return err
    }
    
    return es.Append(ctx, filter, maxSeq, lendEvent)
}
```

## Error Handling and Concurrency

Handle optimistic concurrency conflicts with retries:

```go
func withRetry(operation func() error, maxRetries int) error {
    for i := 0; i < maxRetries; i++ {
        err := operation()
        if err == nil {
            return nil
        }
        
        // Retry on concurrency conflicts
        if errors.Is(err, eventstore.ErrConcurrencyConflict) {
            time.Sleep(time.Duration(1<<i) * 10 * time.Millisecond)
            continue
        }
        
        return err // Non-retryable error
    }
    return errors.New("max retries exceeded")
}

// Usage
err := withRetry(func() error {
    return LendBookToReader(ctx, es, bookID, readerID)
}, 3)
```

## Testing with Different Adapters

```go
func TestLendBookToReader(t *testing.T) {
    ctx := context.Background()
    
    // Test database setup handles ADAPTER_TYPE env var
    db := setupTestDB(t) 
    es := postgresengine.NewEventStoreFromPGXPool(db) // or another adapter
    
    // Test the lending use case
    err := LendBookToReader(ctx, es, "book-123", "reader-456") 
    assert.NoError(t, err)
}
```