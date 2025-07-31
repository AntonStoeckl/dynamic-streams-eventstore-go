# Core Concepts

## What are Dynamic Event Streams?

Dynamic Event Streams represent a fundamental shift from traditional Event Sourcing approaches.  
Instead of fixed streams tied to specific entities, this approach allows you to define consistency boundaries dynamically based on your business needs.


### Traditional vs. Dynamic Event Streams

**Traditional Event Sourcing:**
```
BookCirculation Entity: [BookCopyAddedToCirculation, BookCopyRemovedFromCirculation, ...]
ReaderAccount Entity:   [ReaderRegistered, ReaderContractCanceled, ...]
BookLending Entity:     [BookCopyLentToReader, BookCopyReturnedByReader, ...]
```

**Dynamic Event Streams:**
```
Entity-independent Stream: [BookCopyAddedToCirculation, BookCopyRemovedFromCirculation, ReaderRegistered, 
                            ReaderContractCanceled, BookCopyLentToReader, BookCopyReturnedByReader, ...]
                           ↑ Query events spanning multiple entities
```

### Dynamic Consistency Boundaries vs. Dynamic Event Streams

**Dynamic Consistency Boundaries (DCB)** work by tagging each event as belonging to one or multiple dynamic streams.  
**Dynamic Event Streams** eliminate the need for stream tagging entirely by querying events directly based on JSON payload predicates.  
This reduces complexity and eliminates the need to think in terms of fixed entities or streams.

## The Problem Dynamic Event Streams Solve

Consider these business scenarios:

1. **Book Lending**: When a reader borrows a book, you need to:
   - Verify the book is in circulation and is not currently lent out
   - Verify that the reader (still) has an uncanceled contract
   - Check the reader's borrowing quota
   - Update both circulation and lending streams

2. **Student Enrollment**: When enrolling a student in a course:
   - Verify course capacity
   - Check student prerequisites
   - Update both student and course streams

Traditional event stores force you to either:
- Accept eventual consistency (complex saga patterns)
- Model everything as a single large entity (performance issues)
- Or find other creative/complex solutions

While reading multiple streams to make the business decision is relatively easy, apart from performance considerations,
the consistency boundary of traditional, stream-based event stores is **one** stream, so it's typically not possible to atomically 
append to multiple streams.  
And even if that is possible (at least one vendor is working on it), if you append multiple events about the same fact to multiple streams,
then there is no single source of truth anymore.

Last but not least: **Simplicity** trumps *clever* solutions and makes Event Sourcing more approachable!

## How Dynamic Streams Work

### 1. Query Phase
Build a filter that spans multiple entities:

```go
filter := BuildEventFilter().
    Matching().
    AnyEventTypeOf(
        "BookCopyAddedToCirculation",
        "BookCopyRemovedFromCirculation",
        "ReaderRegistered",
        "ReaderContractCanceled",
        "BookCopyLentToReader", 
        "BookCopyReturnedByReader").
    AndAnyPredicateOf(
        P("BookID", bookID),
        P("ReaderID", readerID)).
    Finalize()
```

This queries ALL events that affect either the book OR the reader.

### 2. Business Logic Phase
Apply your business rules to the complete event history:

```go
events, maxSeqNum, _ := eventStore.Query(ctx, filter)

// Convert to domain events and apply business logic
domainEvents := convertToDomainEvents(events)
newEvent, err := applyBusinessLogic(domainEvents)
```

### 3. Atomic Append Phase
The magic happens here. The same filter used for querying is used for appending:

```sql
WITH context AS (
    SELECT MAX(sequence_number) AS max_seq
    FROM events 
    WHERE -- SAME WHERE CLAUSE AS QUERY
)
INSERT INTO events (...)
SELECT ...
FROM context 
WHERE (COALESCE(max_seq, 0) = 42) -- Expected version
```

If the event stream changed since your query (someone else added events), the append operation fails with a concurrency error.

## Key Benefits

### 1. True ACID Transactions
- No distributed transactions needed
- No saga complexity
- PostgreSQL guarantees atomicity

### 2. Flexible Consistency Boundaries
- Define boundaries per use case, not per entity
- Entity-independent operations become simple
- Business logic stays pure and testable

### 3. Performance
- Single database transaction
- Optimized queries with JSON indexes
- No coordination overhead

### 4. Simplified Architecture
- No message buses required for consistency
- Fewer moving parts
- Clear separation of concerns

## Implementation Details

### Filter Building
The fluent FilterBuilder ensures only valid filter combinations:

```go
// Valid: Events for specific entities
BuildEventFilter().
    Matching().
    AnyPredicateOf(P("BookID", bookID)).
    Finalize()

// Valid: Specific event types across entities  
BuildEventFilter().
    Matching().
    AnyEventTypeOf("BookCopyLentToReader", "BookCopyReturnedByReader").
    AndAnyPredicateOf(
        P("BookID", bookID),
        P("ReaderID", readerID)).
    Finalize()
```

### Optimistic Concurrency Control
The system uses the highest sequence number as a version:

1. Query returns events + `maxSequenceNumber`
2. Business logic processes events
3. Append uses the same filter + `maxSequenceNumber` 
4. If stream changed, append fails → retry from step 1

### JSON-Based Predicates
Events are stored as JSONB, enabling flexible queries:

```go
P("BookID", "123")        // payload @> '{"BookID": "123"}'
P("ReaderType", "Premium") // payload @> '{"ReaderType": "Premium"}'
```

## When to Use Dynamic Streams

**✅ Great for:**
- Entity-independent business rules
- Complex consistency requirements
- High-frequency updates to related entities
- Replacing complex sagas

**❌ Consider alternatives for:**
- Simple CRUD operations
- Truly independent entities
- High-scale scenarios requiring partitioning

## Comparison with Other Patterns

| Pattern             | Consistency  | Complexity | Performance | Scalability |
|---------------------|--------------|------------|-------------|-------------|
| **Dynamic Streams** | Strong       | Low        | High        | Medium      |
| **Traditional ES**  | Entity-level | Medium     | High        | High        |  
| **Sagas**           | Eventual     | High       | Medium      | High        |
| **2PC/XA**          | Strong       | High       | Low         | Low         |

## Next Steps

- See [Usage Examples](./usage-examples.md) for practical implementations
- Review [Performance](./performance.md) characteristics and benchmarks
- Check [Development Guide](./development.md) for adopting in existing systems