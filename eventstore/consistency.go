package eventstore

import "context"

// ConsistencyLevel defines the consistency requirements for EventStore operations.
type ConsistencyLevel int

const (
	// StrongConsistency requires reads from the primary database to ensure
	// read-after-write consistency. This is the default for EventStore operations
	// to provide safety in event sourcing scenarios where command handlers need
	// to see their own writes immediately.
	StrongConsistency ConsistencyLevel = iota

	// EventualConsistency allows reads from replica databases, trading consistency
	// for performance. Suitable for pure query operations that can tolerate
	// slightly stale data in exchange for a reduced load on the primary database.
	EventualConsistency
)

// contextKey is a private type to prevent context key collisions.
type contextKey string

// ConsistencyLevelKey is the context key used to store consistency level preferences.
const ConsistencyLevelKey contextKey = "eventstore.consistency_level"

// WithStrongConsistency returns a context that signals EventStore operations
// should use the primary database for strong consistency guarantees.
//
// This is typically used by command handlers that perform read-check-write
// patterns and need to ensure they see the most recent state.
//
// Example usage:
//
//	ctx = eventstore.WithStrongConsistency(ctx)
//	events, maxSeq, err := eventStore.Query(ctx, filter)
func WithStrongConsistency(ctx context.Context) context.Context {
	return context.WithValue(ctx, ConsistencyLevelKey, StrongConsistency)
}

// WithEventualConsistency returns a context that signals EventStore operations
// may use replica databases for eventual consistency, trading consistency for
// performance.
//
// This is typically used by pure query handlers that can tolerate slightly
// stale data in exchange for better performance and reduced primary database load.
//
// Example usage:
//
//	ctx = eventstore.WithEventualConsistency(ctx)
//	events, maxSeq, err := eventStore.Query(ctx, filter)
func WithEventualConsistency(ctx context.Context) context.Context {
	return context.WithValue(ctx, ConsistencyLevelKey, EventualConsistency)
}

// GetConsistencyLevel extracts the consistency level from the context.
// If no consistency level is set, it returns StrongConsistency as the safe default
// for event sourcing scenarios.
func GetConsistencyLevel(ctx context.Context) ConsistencyLevel {
	if level, ok := ctx.Value(ConsistencyLevelKey).(ConsistencyLevel); ok {
		return level
	}
	return StrongConsistency // Safe default for event sourcing
}

// String provides a string representation of ConsistencyLevel for logging and debugging.
func (c ConsistencyLevel) String() string {
	switch c {
	case StrongConsistency:
		return "strong"
	case EventualConsistency:
		return "eventual"
	default:
		return "unknown"
	}
}
