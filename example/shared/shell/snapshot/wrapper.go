package snapshot

import (
	"context"
	"errors"
	"fmt"
	"time"

	jsoniter "github.com/json-iterator/go"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell"
)

const (
	// snapshotSaveTimeout is the timeout for snapshot save operations to prevent hanging.
	snapshotSaveTimeout = 60 * time.Second
)

var (
	// ErrEventStoreNotSnapshotCapable is returned when the base handler's EventStore doesn't support snapshot operations.
	ErrEventStoreNotSnapshotCapable = errors.New("base handler's EventStore does not support snapshot operations")
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

// GenericSnapshotWrapper provides snapshot-based optimization for any query handler.
// It wraps a base handler and adds incremental projection capabilities using snapshots.
// The wrapper attempts to load existing snapshots, perform incremental updates, and save
// updated snapshots, falling back to the base handler if snapshots are unavailable.
type GenericSnapshotWrapper[Q shell.Query, R shell.QueryResult] struct {
	baseHandler      shell.QueryHandler[Q, R]
	eventStore       QueriesEventsAndHandlesSnapshots
	projectFunc      shell.ProjectionFunc[Q, R]
	filterBuilder    shell.FilterBuilderFunc[Q]
	metricsCollector shell.MetricsCollector
	tracingCollector shell.TracingCollector
	contextualLogger shell.ContextualLogger
	logger           shell.Logger
}

// NewGenericSnapshotWrapper creates a new snapshot-aware wrapper around the base query handler.
// The wrapper will attempt to use snapshots for performance optimization but will fall back
// to the base handler if snapshots are not available or incompatible.
// Observability components are automatically extracted from the base handler.
// Returns an error if the base handler's EventStore doesn't support snapshot operations.
func NewGenericSnapshotWrapper[Q shell.Query, R shell.QueryResult](
	baseHandler shell.QueryHandler[Q, R],
	projectFunc shell.ProjectionFunc[Q, R],
	filterBuilder shell.FilterBuilderFunc[Q],
) (*GenericSnapshotWrapper[Q, R], error) {
	baseEventStore := baseHandler.ExposeEventStore()

	snapshotCapableEventStore, ok := baseEventStore.(QueriesEventsAndHandlesSnapshots)
	if !ok {
		return nil, errors.Join(
			ErrEventStoreNotSnapshotCapable,
			fmt.Errorf("EventStore type %T does not support snapshots", baseEventStore),
		)
	}

	wrapper := &GenericSnapshotWrapper[Q, R]{
		baseHandler:      baseHandler,
		eventStore:       snapshotCapableEventStore,
		projectFunc:      projectFunc,
		filterBuilder:    filterBuilder,
		metricsCollector: baseHandler.ExposeMetricsCollector(),
		tracingCollector: baseHandler.ExposeTracingCollector(),
		contextualLogger: baseHandler.ExposeContextualLogger(),
		logger:           baseHandler.ExposeLogger(),
	}

	return wrapper, nil
}

// Handle executes the snapshot-aware query processing workflow.
// It attempts to load an existing snapshot and perform incremental updates,
// falling back to the base handler if snapshots are unavailable or incompatible.
func (w *GenericSnapshotWrapper[Q, R]) Handle(ctx context.Context, query Q) (R, error) {
	// Start query handler instrumentation
	queryStart := time.Now()
	queryType := query.QueryType()
	ctx, span := shell.StartQuerySpan(ctx, w.tracingCollector, queryType)
	shell.LogQueryStart(ctx, w.logger, w.contextualLogger, queryType)

	// Get the base filter for this query
	baseFilter := w.filterBuilder(query)

	// Snapshot Load phase
	snapshot, err := w.executeSnapshotLoad(ctx, query, baseFilter)
	if err != nil {
		return w.recordFallbackAndExecute(ctx, query, queryStart, span, shell.SnapshotReasonError)
	}

	// Fall back and then save the data as a snapshot
	if snapshot == nil {
		return w.recordFallbackAndExecute(ctx, query, queryStart, span, shell.SnapshotReasonMiss)
	}

	// Reopen filter for sequence filtering (compile-time check)
	reopened := baseFilter.ReopenForSequenceFiltering()
	sequenceCapableFilter := reopened.(eventstore.SequenceFilteringCapable)

	// Incremental Query phase
	storableEvents, maxSeq, err := w.executeIncrementalQuery(ctx, queryType, sequenceCapableFilter, snapshot.SequenceNumber)
	if err != nil {
		return w.recordFallbackAndExecute(ctx, query, queryStart, span, shell.SnapshotReasonIncrementalQueryError)
	}

	// Unmarshal phase
	incrementalEvents, err := w.executeUnmarshal(ctx, queryType, storableEvents)
	if err != nil {
		return w.recordFallbackAndExecute(ctx, query, queryStart, span, shell.SnapshotReasonUnmarshalError)
	}

	// Snapshot Deserialization phase
	baseProjection, err := w.executeSnapshotDeserialization(ctx, queryType, snapshot)
	if err != nil {
		return w.recordFallbackAndExecute(ctx, query, queryStart, span, shell.SnapshotReasonDeserializeError)
	}

	// Determine the final sequence number (max of snapshot and incremental query)
	finalSequence := maxSeq
	if snapshot.SequenceNumber > finalSequence {
		finalSequence = snapshot.SequenceNumber
	}

	// Incremental Projection phase
	result := w.executeIncrementalProjection(ctx, query, incrementalEvents, baseProjection, finalSequence)

	// Save the updated snapshot with incremental changes
	w.saveUpdatedSnapshot(ctx, query, baseFilter, finalSequence, result)

	w.logSnapshotHit(ctx, snapshot.SequenceNumber, maxSeq, len(incrementalEvents))
	w.recordQuerySuccess(ctx, query, time.Since(queryStart), span, shell.SnapshotReasonHit)

	return result, nil
}

// BuildSnapshotType returns the snapshot type string for this handler.
// Tests use this to query for saved snapshots.
func (w *GenericSnapshotWrapper[Q, R]) BuildSnapshotType(query Q) string {
	return query.SnapshotType()
}

/*** Phase execution methods for clean observability patterns ***/

// executeSnapshotLoad handles the snapshot loading phase with proper observability.
func (w *GenericSnapshotWrapper[Q, R]) executeSnapshotLoad(
	ctx context.Context,
	query Q,
	filter eventstore.Filter,
) (*eventstore.Snapshot, error) {
	queryType := query.QueryType()
	snapshotLoadStart := time.Now()
	snapshotType := query.SnapshotType()
	snapshot, err := w.eventStore.LoadSnapshot(ctx, snapshotType, filter)
	snapshotLoadDuration := time.Since(snapshotLoadStart)

	if err != nil {
		// Actual error (not just "not found")
		w.recordComponentTiming(ctx, queryType, shell.ComponentSnapshotLoad, shell.StatusError, snapshotLoadDuration)
		if w.contextualLogger != nil {
			w.contextualLogger.ErrorContext(ctx, "snapshot load error", shell.LogAttrError, err.Error())
		} else if w.logger != nil {
			w.logger.Error("snapshot load error", shell.LogAttrError, err.Error())
		}
		return nil, err
	}

	if snapshot == nil {
		// Snapshot isn't found (normal case)
		w.recordComponentTiming(ctx, queryType, shell.ComponentSnapshotLoad, shell.StatusError, snapshotLoadDuration)
		if w.contextualLogger != nil {
			w.contextualLogger.InfoContext(ctx, shell.LogMsgSnapshotMiss)
		} else if w.logger != nil {
			w.logger.Info(shell.LogMsgSnapshotMiss)
		}
		return nil, nil
	}

	// Snapshot found successfully
	w.recordComponentTiming(ctx, queryType, shell.ComponentSnapshotLoad, shell.StatusSuccess, snapshotLoadDuration)

	return snapshot, nil
}

// executeIncrementalQuery handles the incremental query phase with proper observability.
func (w *GenericSnapshotWrapper[Q, R]) executeIncrementalQuery(
	ctx context.Context,
	queryType string,
	capable eventstore.SequenceFilteringCapable,
	fromSequence uint,
) (eventstore.StorableEvents, eventstore.MaxSequenceNumberUint, error) {
	incrementalQueryStart := time.Now()
	incrementalFilter := capable.WithSequenceNumberHigherThan(fromSequence).Finalize()
	storableEvents, maxSeq, err := w.eventStore.Query(ctx, incrementalFilter)
	incrementalQueryDuration := time.Since(incrementalQueryStart)

	if err != nil {
		w.recordComponentTiming(ctx, queryType, shell.ComponentIncrementalQuery, shell.StatusError, incrementalQueryDuration)
		if w.contextualLogger != nil {
			w.contextualLogger.ErrorContext(ctx, shell.LogMsgIncrementalQueryError, shell.LogAttrError, err.Error())
		} else if w.logger != nil {
			w.logger.Error(shell.LogMsgIncrementalQueryError, shell.LogAttrError, err.Error())
		}
		return nil, 0, err
	}

	w.recordComponentTiming(ctx, queryType, shell.ComponentIncrementalQuery, shell.StatusSuccess, incrementalQueryDuration)

	return storableEvents, maxSeq, nil
}

// executeUnmarshal handles the event unmarshaling phase with proper observability.
func (w *GenericSnapshotWrapper[Q, R]) executeUnmarshal(
	ctx context.Context,
	queryType string,
	storableEvents eventstore.StorableEvents,
) (core.DomainEvents, error) {
	unmarshalStart := time.Now()
	incrementalEvents, err := shell.DomainEventsFrom(storableEvents)
	unmarshalDuration := time.Since(unmarshalStart)

	if err != nil {
		w.recordComponentTiming(ctx, queryType, shell.ComponentUnmarshal, shell.StatusError, unmarshalDuration)
		if w.contextualLogger != nil {
			w.contextualLogger.ErrorContext(ctx, shell.LogMsgEventConversionError, shell.LogAttrError, err.Error())
		} else if w.logger != nil {
			w.logger.Error(shell.LogMsgEventConversionError, shell.LogAttrError, err.Error())
		}
		return nil, err
	}

	w.recordComponentTiming(ctx, queryType, shell.ComponentUnmarshal, shell.StatusSuccess, unmarshalDuration)

	return incrementalEvents, nil
}

// executeSnapshotDeserialization handles snapshot deserialization with proper observability.
func (w *GenericSnapshotWrapper[Q, R]) executeSnapshotDeserialization(
	ctx context.Context,
	queryType string,
	snapshot *eventstore.Snapshot,
) (R, error) {
	deserializationStart := time.Now()
	var baseProjection R
	err := jsoniter.ConfigFastest.Unmarshal(snapshot.Data, &baseProjection)
	deserializationDuration := time.Since(deserializationStart)

	if err != nil {
		w.recordComponentTiming(ctx, queryType, shell.ComponentSnapshotDeserialize, shell.StatusError, deserializationDuration)
		if w.contextualLogger != nil {
			w.contextualLogger.ErrorContext(ctx, shell.LogMsgSnapshotDeserializationError, shell.LogAttrError, err.Error())
		} else if w.logger != nil {
			w.logger.Error(shell.LogMsgSnapshotDeserializationError, shell.LogAttrError, err.Error())
		}
		return baseProjection, err
	}

	w.recordComponentTiming(ctx, queryType, shell.ComponentSnapshotDeserialize, shell.StatusSuccess, deserializationDuration)

	return baseProjection, nil
}

// executeIncrementalProjection handles incremental projection with proper observability.
func (w *GenericSnapshotWrapper[Q, R]) executeIncrementalProjection(
	ctx context.Context,
	query Q,
	incrementalEvents core.DomainEvents,
	baseProjection R,
	maxSequence eventstore.MaxSequenceNumberUint,
) R {
	queryType := query.QueryType()
	incrementalProjectionStart := time.Now()
	result := w.projectFunc(incrementalEvents, query, maxSequence, baseProjection)
	incrementalProjectionDuration := time.Since(incrementalProjectionStart)

	w.recordComponentTiming(ctx, queryType, shell.ComponentIncrementalProjection, shell.StatusSuccess, incrementalProjectionDuration)

	return result
}

// saveUpdatedSnapshot saves the updated projection as a snapshot.
// This is called synchronously to ensure reliable snapshot storage.
// Uses a background context with timeout to avoid cancellation issues.
func (w *GenericSnapshotWrapper[Q, R]) saveUpdatedSnapshot(
	parentCtx context.Context,
	query Q,
	filter eventstore.Filter,
	maxSequence eventstore.MaxSequenceNumberUint,
	projection R,
) {
	// Create context with additional timeout for snapshot saving, inheriting cancellation
	ctx, cancel := context.WithTimeout(parentCtx, snapshotSaveTimeout)
	defer cancel()

	// Start instrumentation for the entire snapshot save operation
	snapshotSaveStart := time.Now()

	// Serialize projection to JSON
	queryType := query.QueryType()
	data, err := jsoniter.ConfigFastest.Marshal(projection)
	if err != nil {
		w.recordSnapshotSaveError(ctx, queryType, "JSON serialization", err, snapshotSaveStart)
		return
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
		w.recordSnapshotSaveError(ctx, queryType, "snapshot build", err, snapshotSaveStart)
		return
	}

	// Save snapshot
	if err := w.eventStore.SaveSnapshot(ctx, snapshot); err != nil {
		w.recordSnapshotSaveError(ctx, queryType, "snapshot save", err, snapshotSaveStart)
		return
	}

	// Record successful snapshot save
	w.recordComponentTiming(ctx, queryType, shell.ComponentSnapshotSave, shell.StatusSuccess, time.Since(snapshotSaveStart))

	if w.contextualLogger != nil {
		w.contextualLogger.InfoContext(ctx, shell.LogMsgSnapshotSaved, shell.LogAttrSequence, maxSequence)
	} else if w.logger != nil {
		w.logger.Info(shell.LogMsgSnapshotSaved, shell.LogAttrSequence, maxSequence)
	}
}

/*** Observability helper methods ***/

// recordQuerySuccess records successful snapshot-aware query execution with observability.
func (w *GenericSnapshotWrapper[Q, R]) recordQuerySuccess(
	ctx context.Context,
	query Q,
	duration time.Duration,
	span shell.SpanContext,
	snapshotStatus string,
) {
	queryType := query.QueryType()
	shell.RecordQueryMetrics(ctx, w.metricsCollector, queryType, shell.StatusSuccess, duration)
	shell.FinishQuerySpan(w.tracingCollector, span, shell.StatusSuccess, duration, nil)

	// Log success with snapshot status for better observability
	if w.contextualLogger != nil {
		w.contextualLogger.InfoContext(ctx, shell.LogMsgSnapshotQuerySuccess,
			shell.LogAttrQueryType, queryType,
			shell.LogAttrStatus, shell.StatusSuccess,
			shell.LogAttrDurationMS, duration.Milliseconds(),
			shell.LogAttrSnapshotStatus, snapshotStatus)
	} else if w.logger != nil {
		w.logger.Info(shell.LogMsgSnapshotQuerySuccess,
			shell.LogAttrQueryType, queryType,
			shell.LogAttrStatus, shell.StatusSuccess,
			shell.LogAttrDurationMS, duration.Milliseconds(),
			shell.LogAttrSnapshotStatus, snapshotStatus)
	}
}

// recordFallbackAndExecute records the fallback scenario and delegates to base handler.
func (w *GenericSnapshotWrapper[Q, R]) recordFallbackAndExecute(
	ctx context.Context,
	query Q,
	queryStart time.Time,
	span shell.SpanContext,
	fallbackReason string,
) (R, error) {
	duration := time.Since(queryStart)
	queryType := query.QueryType()

	// Record the fallback as a successful operation (since base handler will handle it)
	shell.RecordQueryMetrics(ctx, w.metricsCollector, queryType, shell.StatusSuccess, duration)
	shell.FinishQuerySpan(w.tracingCollector, span, shell.StatusSuccess, duration, nil)

	if w.contextualLogger != nil {
		w.contextualLogger.InfoContext(ctx, shell.LogMsgSnapshotFallback,
			shell.LogAttrReason, fallbackReason,
			shell.LogAttrDurationMS, duration.Milliseconds())
	} else if w.logger != nil {
		w.logger.Info(shell.LogMsgSnapshotFallback,
			shell.LogAttrReason, fallbackReason,
			shell.LogAttrDurationMS, duration.Milliseconds())
	}

	// Delegate to base handler
	result, err := w.baseHandler.Handle(ctx, query)
	if err != nil {
		return result, err
	}

	// For snapshot miss, save the result as an initial snapshot for future queries
	if fallbackReason == shell.SnapshotReasonMiss {
		baseFilter := w.filterBuilder(query)
		// Use the sequence number from the result (no double query needed!)
		w.saveUpdatedSnapshot(ctx, query, baseFilter, result.GetSequenceNumber(), result)
	}

	return result, nil
}

// recordComponentTiming records component-level timing metrics for snapshot operations.
func (w *GenericSnapshotWrapper[Q, R]) recordComponentTiming(
	ctx context.Context,
	queryType string,
	component string,
	status string,
	duration time.Duration,
) {
	shell.RecordQueryComponentDuration(ctx, w.metricsCollector, queryType, component, status, duration)
}

// recordSnapshotSaveError handles error recording and logging for snapshot save operations.
func (w *GenericSnapshotWrapper[Q, R]) recordSnapshotSaveError(
	ctx context.Context,
	queryType string,
	operation string,
	err error,
	startTime time.Time,
) {
	w.recordComponentTiming(ctx, queryType, shell.ComponentSnapshotSave, shell.StatusError, time.Since(startTime))

	if w.contextualLogger != nil {
		w.contextualLogger.ErrorContext(ctx, shell.LogMsgSnapshotSaveError,
			shell.LogAttrOperation, operation,
			shell.LogAttrError, err.Error())
	} else if w.logger != nil {
		w.logger.Error(shell.LogMsgSnapshotSaveError,
			shell.LogAttrOperation, operation,
			shell.LogAttrError, err.Error())
	}
}

// logSnapshotHit logs a successful snapshot hit with sequence information.
func (w *GenericSnapshotWrapper[Q, R]) logSnapshotHit(
	ctx context.Context,
	fromSequence uint,
	toSequence eventstore.MaxSequenceNumberUint,
	eventCount int,
) {
	if w.contextualLogger != nil {
		w.contextualLogger.InfoContext(ctx, shell.LogMsgSnapshotHit,
			shell.LogAttrFromSequence, fromSequence,
			shell.LogAttrToSequence, toSequence,
			shell.LogAttrEventCount, eventCount)
	} else if w.logger != nil {
		w.logger.Info(shell.LogMsgSnapshotHit,
			shell.LogAttrFromSequence, fromSequence,
			shell.LogAttrToSequence, toSequence,
			shell.LogAttrEventCount, eventCount)
	}
}
