# Usage Examples

This document provides practical examples of using dynamic-streams-eventstore-go in real-world scenarios.

## Example 1: Library Book Lending System

This example demonstrates cross-entity operations where a single business transaction affects multiple entities.

### Domain Events

```go
package events

import "time"

type BookCopyAddedToCirculation struct {
    BookID       string    `json:"BookID"`
    ISBN         string    `json:"ISBN"`
    Title        string    `json:"Title"`
    Authors      string    `json:"Authors"`
    OccurredAt   time.Time `json:"OccurredAt"`
}

func (e BookCopyAddedToCirculation) EventType() string {
    return "BookCopyAddedToCirculation"
}

func (e BookCopyAddedToCirculation) HasOccurredAt() time.Time {
    return e.OccurredAt
}

type BookCopyLentToReader struct {
    BookID     string    `json:"BookID"`
    ReaderID   string    `json:"ReaderID"`
    OccurredAt time.Time `json:"OccurredAt"`
}

func (e BookCopyLentToReader) EventType() string {
    return "BookCopyLentToReader"
}

func (e BookCopyLentToReader) HasOccurredAt() time.Time {
    return e.OccurredAt
}

type BookCopyReturnedByReader struct {
    BookID     string    `json:"BookID"`
    ReaderID   string    `json:"ReaderID"`
    OccurredAt time.Time `json:"OccurredAt"`
}

func (e BookCopyReturnedByReader) EventType() string {
    return "BookCopyReturnedByReader"
}

func (e BookCopyReturnedByReader) HasOccurredAt() time.Time {
    return e.OccurredAt
}
```

### Business Logic

```go
package business

import (
    "errors"
    "fmt"
)

type BookState struct {
    BookID    string
    Available bool
    Title     string
}

type ReaderState struct {
    ReaderID      string
    BorrowedBooks int
    MaxBooks      int
}

var (
    ErrBookNotAvailable = errors.New("book is not available")
    ErrReaderQuotaExceeded = errors.New("reader has exceeded borrowing quota")
    ErrBookNotBorrowed = errors.New("book is not currently borrowed by this reader")
)

func CanLendBookToReader(bookState BookState, readerState ReaderState) error {
    if !bookState.Available {
        return ErrBookNotAvailable
    }
    
    if readerState.BorrowedBooks >= readerState.MaxBooks {
        return ErrReaderQuotaExceeded
    }
    
    return nil
}

func CanReturnBook(bookState BookState, readerState ReaderState, readerID string) error {
    if bookState.Available {
        return ErrBookNotBorrowed
    }
    
    // Additional logic to verify this reader actually borrowed the book
    // would require checking the lending history
    
    return nil
}
```

### Event Processing

```go
package projection

import "github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"

func BuildBookAndReaderState(events eventstore.StorableEvents) (BookState, ReaderState, error) {
    var bookState BookState
    var readerState ReaderState
    
    // Set defaults
    readerState.MaxBooks = 5 // Default quota
    bookState.Available = false
    
    for _, event := range events {
        switch event.EventType {
        case "BookCopyAddedToCirculation":
            var e BookCopyAddedToCirculation
            if err := json.Unmarshal(event.PayloadJSON, &e); err != nil {
                return bookState, readerState, err
            }
            bookState.BookID = e.BookID
            bookState.Title = e.Title
            bookState.Available = true
            
        case "BookCopyLentToReader":
            var e BookCopyLentToReader
            if err := json.Unmarshal(event.PayloadJSON, &e); err != nil {
                return bookState, readerState, err
            }
            if e.BookID == bookState.BookID {
                bookState.Available = false
            }
            if e.ReaderID == readerState.ReaderID {
                readerState.BorrowedBooks++
            }
            
        case "BookCopyReturnedByReader":
            var e BookCopyReturnedByReader
            if err := json.Unmarshal(event.PayloadJSON, &e); err != nil {
                return bookState, readerState, err
            }
            if e.BookID == bookState.BookID {
                bookState.Available = true
            }
            if e.ReaderID == readerState.ReaderID {
                readerState.BorrowedBooks--
            }
        }
    }
    
    return bookState, readerState, nil
}
```

### Complete Use Case Implementation

```go
package usecases

import (
    "context"
    "encoding/json"
    "time"
    
    . "github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
    "github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/engine"
)

type LendBookCommand struct {
    BookID   string
    ReaderID string
}

func LendBookToReader(
    ctx context.Context,
    es engine.PostgresEventStore,
    cmd LendBookCommand,
) error {
    // 1. Build filter for cross-entity query
    filter := BuildEventFilter().
        Matching().
        AnyEventTypeOf(
            "BookCopyAddedToCirculation",
            "BookCopyLentToReader",
            "BookCopyReturnedByReader").
        AndAnyPredicateOf(
            P("BookID", cmd.BookID),
            P("ReaderID", cmd.ReaderID)).
        Finalize()
    
    // 2. Query current state
    events, maxSeqNum, err := es.Query(ctx, filter)
    if err != nil {
        return err
    }
    
    // 3. Build current state from events
    bookState, readerState, err := BuildBookAndReaderState(events)
    if err != nil {
        return err
    }
    
    // 4. Apply business rules
    if err := CanLendBookToReader(bookState, readerState); err != nil {
        return err
    }
    
    // 5. Create new event
    lendingEvent := BookCopyLentToReader{
        BookID:     cmd.BookID,
        ReaderID:   cmd.ReaderID,
        OccurredAt: time.Now(),
    }
    
    payloadJSON, err := json.Marshal(lendingEvent)
    if err != nil {
        return err
    }
    
    storableEvent := BuildStorableEventWithEmptyMetadata(
        lendingEvent.EventType(),
        lendingEvent.HasOccurredAt(),
        payloadJSON,
    )
    
    // 6. Append atomically with optimistic locking
    return es.Append(ctx, filter, maxSeqNum, storableEvent)
}
```

## Example 2: E-commerce Order Processing

### Events for Order Management

```go
type ProductAddedToCatalog struct {
    ProductID   string `json:"ProductID"`
    Name        string `json:"Name"`
    Price       int    `json:"Price"` // in cents
    StockLevel  int    `json:"StockLevel"`
    OccurredAt  time.Time `json:"OccurredAt"`
}

type StockReserved struct {
    ProductID   string `json:"ProductID"`
    OrderID     string `json:"OrderID"`
    Quantity    int    `json:"Quantity"`
    OccurredAt  time.Time `json:"OccurredAt"`
}

type OrderPlaced struct {
    OrderID     string `json:"OrderID"`
    CustomerID  string `json:"CustomerID"`
    ProductID   string `json:"ProductID"`
    Quantity    int    `json:"Quantity"`
    OccurredAt  time.Time `json:"OccurredAt"`
}
```

### Cross-Entity Business Logic

```go
func PlaceOrder(
    ctx context.Context,
    es engine.PostgresEventStore,
    customerID, productID string,
    quantity int,
) error {
    orderID := generateOrderID()
    
    // Query both product and customer events
    filter := BuildEventFilter().
        Matching().
        AnyEventTypeOf(
            "ProductAddedToCatalog",
            "StockReserved",
            "OrderPlaced").
        AndAnyPredicateOf(
            P("ProductID", productID),
            P("CustomerID", customerID)).
        Finalize()
    
    events, maxSeqNum, err := es.Query(ctx, filter)
    if err != nil {
        return err
    }
    
    productState, customerState := buildStateFromEvents(events)
    
    // Business rules
    if productState.AvailableStock < quantity {
        return errors.New("insufficient stock")
    }
    
    if customerState.OrderCount >= customerState.MaxOrders {
        return errors.New("customer order limit exceeded")
    }
    
    // Create multiple events atomically
    events := []StorableEvent{
        toStorableEvent(StockReserved{
            ProductID: productID,
            OrderID: orderID,
            Quantity: quantity,
            OccurredAt: time.Now(),
        }),
        toStorableEvent(OrderPlaced{
            OrderID: orderID,
            CustomerID: customerID,
            ProductID: productID,
            Quantity: quantity,
            OccurredAt: time.Now(),
        }),
    }
    
    return es.AppendMultiple(ctx, filter, maxSeqNum, events)
}
```

## Example 3: Financial Account Transfers

### Events for Money Transfer

```go
type AccountOpened struct {
    AccountID   string `json:"AccountID"`
    CustomerID  string `json:"CustomerID"`
    Currency    string `json:"Currency"`
    OccurredAt  time.Time `json:"OccurredAt"`
}

type MoneyDeposited struct {
    AccountID   string `json:"AccountID"`
    Amount      int64  `json:"Amount"` // in cents
    Reference   string `json:"Reference"`
    OccurredAt  time.Time `json:"OccurredAt"`
}

type MoneyTransferred struct {
    FromAccountID string `json:"FromAccountID"`
    ToAccountID   string `json:"ToAccountID"`
    Amount        int64  `json:"Amount"`
    Reference     string `json:"Reference"`
    OccurredAt    time.Time `json:"OccurredAt"`
}
```

### Atomic Transfer Implementation

```go
func TransferMoney(
    ctx context.Context,
    es engine.PostgresEventStore,
    fromAccountID, toAccountID string,
    amount int64,
    reference string,
) error {
    // Query events for both accounts
    filter := BuildEventFilter().
        Matching().
        AnyEventTypeOf(
            "AccountOpened",
            "MoneyDeposited", 
            "MoneyTransferred").
        AndAnyPredicateOf(
            P("AccountID", fromAccountID),
            P("FromAccountID", fromAccountID),
            P("ToAccountID", fromAccountID),
            P("AccountID", toAccountID),
            P("FromAccountID", toAccountID),
            P("ToAccountID", toAccountID)).
        Finalize()
    
    events, maxSeqNum, err := es.Query(ctx, filter)
    if err != nil {
        return err
    }
    
    fromBalance, toBalance := calculateBalances(events, fromAccountID, toAccountID)
    
    // Business rules
    if fromBalance < amount {
        return errors.New("insufficient funds")
    }
    
    // Create transfer event
    transferEvent := MoneyTransferred{
        FromAccountID: fromAccountID,
        ToAccountID:   toAccountID,
        Amount:        amount,
        Reference:     reference,
        OccurredAt:    time.Now(),
    }
    
    storableEvent := toStorableEvent(transferEvent)
    
    return es.Append(ctx, filter, maxSeqNum, storableEvent)
}
```

## Error Handling and Retries

```go
func withRetry(operation func() error, maxRetries int) error {
    var err error
    for i := 0; i < maxRetries; i++ {
        err = operation()
        if err == nil {
            return nil
        }
        
        // Check if it's a concurrency error
        if errors.Is(err, engine.ErrConcurrencyConflict) {
            // Brief exponential backoff
            time.Sleep(time.Duration(1<<i) * 10 * time.Millisecond)
            continue
        }
        
        // Non-retryable error
        return err
    }
    return err
}

// Usage
err := withRetry(func() error {
    return LendBookToReader(ctx, es, cmd)
}, 3)
```

## Testing Patterns

```go
func TestLendBookToReader(t *testing.T) {
    // Setup test database
    ctx := context.Background()
    db := setupTestDB(t)
    es := engine.NewPostgresEventStore(db)
    
    // Given: A book and reader exist
    bookID := "book-123"
    readerID := "reader-456"
    
    setupEvents := []StorableEvent{
        toStorableEvent(BookCopyAddedToCirculation{
            BookID: bookID,
            Title: "Test Book",
            OccurredAt: time.Now(),
        }),
    }
    
    baseFilter := BuildEventFilter().
        Matching().
        AnyPredicateOf(P("BookID", bookID)).
        Finalize()
    
    err := es.AppendMultiple(ctx, baseFilter, 0, setupEvents)
    require.NoError(t, err)
    
    // When: Lending the book
    cmd := LendBookCommand{
        BookID:   bookID,
        ReaderID: readerID,
    }
    
    err = LendBookToReader(ctx, es, cmd)
    
    // Then: Should succeed
    assert.NoError(t, err)
    
    // And: Book should be marked as lent
    events, _, err := es.Query(ctx, buildQueryFilter(bookID, readerID))
    require.NoError(t, err)
    
    assert.Len(t, events, 2) // Book added + Book lent
    assert.Equal(t, "BookCopyLentToReader", events[1].EventType)
}
```

These examples demonstrate the power of Dynamic Event Streams for handling complex, cross-entity business scenarios while maintaining strong consistency and clean separation of concerns.