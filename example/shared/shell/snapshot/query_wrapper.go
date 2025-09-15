package snapshot

import (
	"context"
	"errors"
	"time"

	jsoniter "github.com/json-iterator/go"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell"
)

const (
	// snapshotSaveTimeout is the timeout for snapshot save operations to prevent hanging.
	snapshotSaveTimeoutPlain = 60 * time.Second
)

var (
	// ErrSnapshotLoadFailed is returned when snapshot loading fails.
	ErrSnapshotLoadFailed = errors.New("snapshot load failed")

	// ErrIncrementalQueryFailed is returned when an incremental query fails.
	ErrIncrementalQueryFailed = errors.New("incremental query failed")

	// ErrEventUnmarshalingFailed is returned when event unmarshaling fails.
	ErrEventUnmarshalingFailed = errors.New("event unmarshaling failed")

	// ErrSnapshotDeserializationFailed is returned when snapshot deserialization fails.
	ErrSnapshotDeserializationFailed = errors.New("snapshot deserialization failed")

	// ErrSnapshotSaveFailed is returned when snapshot save fails.
	ErrSnapshotSaveFailed = errors.New("snapshot save failed")

	// ErrJSONSerializationFailed is returned when JSON serialization fails.
	ErrJSONSerializationFailed = errors.New("JSON serialization failed")

	// ErrSnapshotBuildFailed is returned when the snapshot build fails.
	ErrSnapshotBuildFailed = errors.New("snapshot build failed")

	// ErrInitialSnapshotSaveFailed is returned when the initial snapshot save fails.
	ErrInitialSnapshotSaveFailed = errors.New("initial snapshot save failed")
)

// SavesAndLoadsSnapshots defines the interface needed for snapshot-aware operations.
type SavesAndLoadsSnapshots interface {
	SaveSnapshot(ctx context.Context, snapshot eventstore.Snapshot) error
	LoadSnapshot(ctx context.Context, projectionType string, filter eventstore.Filter) (*eventstore.Snapshot, error)
}

// QueriesEventsAndHandlesSnapshots defines the interface needed for snapshot-aware operations.
// This extends the basic EventStore interface with snapshot management capabilities.
type QueriesEventsAndHandlesSnapshots interface {
	shell.QueriesEvents
	SavesAndLoadsSnapshots
}

// QueryWrapper provides snapshot-based optimization for query handlers.
// It wraps a base handler and adds incremental projection capabilities using snapshots.
// The wrapper attempts to load existing snapshots, perform incremental updates, and save
// updated snapshots, falling back to the base handler if snapshots are unavailable.
// This version is completely observability-free - no logging, metrics, or tracing.
type QueryWrapper[Q shell.Query, R shell.QueryResult] struct {
	coreHandler   shell.QueryHandler[Q, R]
	eventStore    QueriesEventsAndHandlesSnapshots
	projectFunc   shell.ProjectionFunc[Q, R]
	filterBuilder shell.FilterBuilderFunc[Q]
}

// NewQueryWrapper creates a new snapshot-aware wrapper around a query handler.
// The wrapper will attempt to use snapshots for performance optimization but will fall back
// to the base handler if snapshots are not available or incompatible.
// The eventStore must be passed separately and must support snapshot operations.
func NewQueryWrapper[Q shell.Query, R shell.QueryResult](
	baseHandler shell.QueryHandler[Q, R],
	eventStore QueriesEventsAndHandlesSnapshots,
	projectFunc shell.ProjectionFunc[Q, R],
	filterBuilder shell.FilterBuilderFunc[Q],
) (*QueryWrapper[Q, R], error) {

	wrapper := &QueryWrapper[Q, R]{
		coreHandler:   baseHandler,
		eventStore:    eventStore,
		projectFunc:   projectFunc,
		filterBuilder: filterBuilder,
	}

	return wrapper, nil
}

// Handle executes the snapshot-aware query processing workflow.
// It attempts to load an existing snapshot and perform incremental updates.
// Only falls back to the base handler for snapshot miss (normal case).
// All other errors are returned immediately to maintain visibility and debuggability.
func (w *QueryWrapper[Q, R]) Handle(ctx context.Context, query Q) (R, error) {
	// Get the base filter for this query
	baseFilter := w.filterBuilder(query)

	// Snapshot Load phase
	snapshot, err := w.loadSnapshot(ctx, query, baseFilter)
	if err != nil {
		return *new(R), errors.Join(ErrSnapshotLoadFailed, err)
	}

	// Snapshot miss - fallback and save the initial snapshot
	if snapshot == nil {
		return w.fallbackAndSaveSnapshot(ctx, query, baseFilter)
	}

	// Incremental Query phase
	reopenedFilter := baseFilter.ReopenForSequenceFiltering().(eventstore.SequenceFilteringCapable)

	storableEvents, maxSeq, err := w.queryIncrementalEvents(ctx, reopenedFilter, snapshot.SequenceNumber)
	if err != nil {
		return *new(R), errors.Join(ErrIncrementalQueryFailed, err)
	}

	// Unmarshal phase
	incrementalEvents, err := w.unmarshalEvents(ctx, storableEvents)
	if err != nil {
		return *new(R), errors.Join(ErrEventUnmarshalingFailed, err)
	}

	// Snapshot Deserialization phase
	baseProjection, err := w.deserializeSnapshot(ctx, snapshot)
	if err != nil {
		return *new(R), errors.Join(ErrSnapshotDeserializationFailed, err)
	}

	// Determine the final sequence number (max of snapshot and incremental query)
	finalSequence := maxSeq
	if snapshot.SequenceNumber > finalSequence {
		finalSequence = snapshot.SequenceNumber
	}

	// Incremental Projection phase
	result := w.projectFunc(incrementalEvents, query, finalSequence, baseProjection)

	// Save the updated snapshot with incremental changes
	if saveErr := w.saveUpdatedSnapshot(ctx, query, baseFilter, finalSequence, result); saveErr != nil {
		return *new(R), errors.Join(ErrSnapshotSaveFailed, saveErr)
	}

	return result, nil
}

/*** Phase execution methods ***/

// loadSnapshot handles the snapshot loading phase.
func (w *QueryWrapper[Q, R]) loadSnapshot(
	ctx context.Context,
	query Q,
	filter eventstore.Filter,
) (*eventstore.Snapshot, error) {
	snapshotType := query.SnapshotType()
	snapshot, err := w.eventStore.LoadSnapshot(ctx, snapshotType, filter)

	if err != nil {
		return nil, err
	}

	if snapshot == nil {
		return nil, nil
	}

	return snapshot, nil
}

// queryIncrementalEvents handles the incremental query phase.
func (w *QueryWrapper[Q, R]) queryIncrementalEvents(
	ctx context.Context,
	filter eventstore.SequenceFilteringCapable,
	fromSequence uint,
) (eventstore.StorableEvents, eventstore.MaxSequenceNumberUint, error) {
	incrementalFilter := filter.WithSequenceNumberHigherThan(fromSequence).Finalize()
	storableEvents, maxSeq, err := w.eventStore.Query(ctx, incrementalFilter)

	if err != nil {
		return nil, 0, err
	}

	return storableEvents, maxSeq, nil
}

// unmarshalEvents handles the event unmarshaling phase.
func (w *QueryWrapper[Q, R]) unmarshalEvents(
	_ context.Context,
	storableEvents eventstore.StorableEvents,
) (core.DomainEvents, error) {
	incrementalEvents, err := shell.DomainEventsFrom(storableEvents)

	if err != nil {
		return nil, err
	}

	return incrementalEvents, nil
}

// deserializeSnapshot handles snapshot deserialization.
func (w *QueryWrapper[Q, R]) deserializeSnapshot(
	_ context.Context,
	snapshot *eventstore.Snapshot,
) (R, error) {
	var baseProjection R
	err := jsoniter.ConfigFastest.Unmarshal(snapshot.Data, &baseProjection)

	if err != nil {
		return baseProjection, err
	}

	return baseProjection, nil
}

// saveUpdatedSnapshot saves the updated projection as a snapshot.
// This is called synchronously to ensure reliable snapshot storage.
// Uses a background context with timeout to avoid cancellation issues.
// Returns an error if any step of the snapshot saving process fails.
func (w *QueryWrapper[Q, R]) saveUpdatedSnapshot(
	parentCtx context.Context,
	query Q,
	filter eventstore.Filter,
	maxSequence eventstore.MaxSequenceNumberUint,
	projection R,
) error {
	// Create context with additional timeout for snapshot saving, inheriting cancellation
	ctx, cancel := context.WithTimeout(parentCtx, snapshotSaveTimeoutPlain)
	defer cancel()

	// Serialize projection to JSON
	data, err := jsoniter.ConfigFastest.Marshal(projection)
	if err != nil {
		return errors.Join(ErrJSONSerializationFailed, err)
	}

	// Build snapshot
	snapshotType := query.SnapshotType()
	snapshot, err := eventstore.BuildSnapshot(
		snapshotType,
		filter.Hash(),
		maxSequence,
		data,
	)
	if err != nil {
		return errors.Join(ErrSnapshotBuildFailed, err)
	}

	// Save snapshot
	if saveErr := w.eventStore.SaveSnapshot(ctx, snapshot); saveErr != nil {
		return errors.Join(ErrSnapshotSaveFailed, saveErr)
	}

	return nil
}

// fallbackAndSaveSnapshot handles fallback to base handler and saves the result as an initial snapshot.
func (w *QueryWrapper[Q, R]) fallbackAndSaveSnapshot(
	ctx context.Context,
	query Q,
	baseFilter eventstore.Filter,
) (R, error) {
	// Delegate to base handler
	result, err := w.coreHandler.Handle(ctx, query)
	if err != nil {
		return result, err
	}

	// For snapshot miss, save the result as an initial snapshot for future queries
	if err := w.saveUpdatedSnapshot(ctx, query, baseFilter, result.GetSequenceNumber(), result); err != nil {
		return *new(R), errors.Join(ErrInitialSnapshotSaveFailed, err)
	}

	return result, nil
}
