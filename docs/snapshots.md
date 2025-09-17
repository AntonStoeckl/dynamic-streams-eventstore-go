# Snapshot Support

Snapshots enable efficient projection rebuilding by storing computed projection states with sequence number tracking for incremental updates.

## Overview

Instead of replaying all events each time you need a projection, snapshots allow you to:

1. **Store projection state** at a specific sequence number
2. **Load the snapshot** and only process events since that point
3. **Update and save** the new snapshot state

This dramatically improves performance for projections over large event streams—for query handling as well as command handling.

## Core Operations

### Save Snapshot

```go
// Build projection data (any JSON-serializable type)
projectionData := BooksInCirculation{
    Books: []Book{{ID: "book-123", Title: "Event Sourcing Guide"}},
    Count: 1,
}

// Serialize to JSON
data, err := json.Marshal(projectionData)
if err != nil {
    return err
}

// Create snapshot with metadata
snapshot, err := eventstore.BuildSnapshot(
    "BooksInCirculation",           // Projection type
    filter.Hash(),                  // Filter hash for isolation
    maxSequenceNumber,              // Last processed sequence number
    data,                          // Serialized projection state
)
if err != nil {
    return err
}

// Save to event store
err = eventStore.SaveSnapshot(ctx, snapshot)
```

### Load Snapshot

```go
// Load existing snapshot for this projection type and filter
snapshot, err := eventStore.LoadSnapshot(ctx, "BooksInCirculation", filter)
if err != nil {
    return err
}

if snapshot == nil {
    // No snapshot exists - start from beginning
    return buildFullProjection(ctx, filter)
}

// Deserialize snapshot data
var projectionState BooksInCirculation
err = json.Unmarshal(snapshot.Data, &projectionState)
if err != nil {
    return err
}

// Continue from this state...
```

### Delete Snapshot

```go
// Remove snapshot (useful for projection resets)
err := eventStore.DeleteSnapshot(ctx, "BooksInCirculation", filter)
if err != nil {
    return err
}
// Idempotent - no error if snapshot doesn't exist
```

## Complete Projection Pattern

Here's the full pattern for efficient snapshot-based projections:

```go
func BuildBooksInCirculationProjection(ctx context.Context, eventStore EventStore, filter eventstore.Filter) (*BooksInCirculation, error) {
    // 1. Try to load existing snapshot
    snapshot, err := eventStore.LoadSnapshot(ctx, "BooksInCirculation", filter)
    if err != nil {
        return nil, err
    }

    var projection BooksInCirculation
    var fromSequence uint = 0

    // 2. Start from snapshot if available
    if snapshot != nil {
        err = json.Unmarshal(snapshot.Data, &projection)
        if err != nil {
            return nil, err
        }
        fromSequence = snapshot.SequenceNumber
    }

    // 3. Query only events since snapshot
    incrementalFilter := filter.ReopenForSequenceFiltering().(eventstore.SequenceFilteringCapable).
        WithSequenceNumberHigherThan(fromSequence).
        Finalize()

    events, maxSeq, err := eventStore.Query(ctx, incrementalFilter)
    if err != nil {
        return nil, err
    }

    // 4. Apply incremental events to projection
    for _, storableEvent := range events {
        domainEvent, err := DomainEventFrom(storableEvent)
        if err != nil {
            return nil, err
        }

        switch event := domainEvent.(type) {
        case BookCopyAddedToCirculation:
            projection.Books = append(projection.Books, Book{
                ID:    event.BookID,
                Title: event.Title,
            })
            projection.Count++

        case BookCopyRemovedFromCirculation:
            projection.Books = removeBook(projection.Books, event.BookID)
            projection.Count--
        }
    }

    // 5. Save updated snapshot for next time
    updatedData, err := json.Marshal(projection)
    if err != nil {
        return nil, err
    }

    newSnapshot, err := eventstore.BuildSnapshot(
        "BooksInCirculation",
        filter.Hash(),
        maxSeq,
        updatedData,
    )
    if err != nil {
        return nil, err
    }

    err = eventStore.SaveSnapshot(ctx, newSnapshot)
    if err != nil {
        return nil, err // Or log error and continue
    }

    return &projection, nil
}
```

## Filter-Based Isolation

Snapshots are tied to specific filters using filter hashes. This means:

```go
// Different filters = different snapshots
filter1 := BuildEventFilter().Matching().AnyEventTypeOf("BookCopyAddedToCirculation").Finalize()
filter2 := BuildEventFilter().Matching().AnyEventTypeOf("BookCopyLentToReader").Finalize()

// These will be separate snapshots
eventStore.SaveSnapshot(ctx, snapshot1) // Uses filter1.Hash()
eventStore.SaveSnapshot(ctx, snapshot2) // Uses filter2.Hash()
```

## Performance Benefits

- **Incremental Updates**: Only process events since the last snapshot
- **Large Event Stores**: Avoid replaying millions of events
- **Complex Projections**: Store computed results instead of recalculating
- **Multiple Projections**: Each projection type maintains separate snapshots

## Best Practices

1. **Save periodically**: Don't save after every event - batch update
2. **Handle errors gracefully**: Snapshot failures shouldn't break queries
3. **Use meaningful projection types**: Clear names like "BooksInCirculation", "ActiveReaders"
4. **Validate deserialization**: Always check JSON unmarshaling errors
5. **Consider cleanup**: Delete snapshots when projections change structure

## Error Handling

```go
snapshot, err := eventStore.LoadSnapshot(ctx, "BooksInCirculation", filter)
if err != nil {
    // Log error but continue with full projection
    log.Warn("Snapshot load failed, using full projection", "error", err)
    return buildFullProjection(ctx, filter)
}

// Save snapshot with error handling
if err := eventStore.SaveSnapshot(ctx, newSnapshot); err != nil {
    // Log but don't fail the query
    log.Error("Snapshot save failed", "error", err)
}
```

The snapshot system is designed to be additive—your projections work with or without snapshots but perform much better when snapshots are available.