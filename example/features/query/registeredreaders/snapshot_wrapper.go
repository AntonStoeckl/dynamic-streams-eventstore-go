package registeredreaders

import (
	"context"
	"errors"
	"time"

	jsoniter "github.com/json-iterator/go"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell"
)

// Sentinel errors for snapshot-aware query handler.
var (
	// ErrEventStoreNotSnapshotCapable is returned when the base handler's EventStore doesn't support snapshot operations.
	ErrEventStoreNotSnapshotCapable = errors.New("base handler's EventStore does not support snapshot operations")

	// ErrFilterNotSequenceCapable is returned when a filter is incompatible with sequence filtering.
	ErrFilterNotSequenceCapable = errors.New("filter incompatible with sequence filtering")
)

// SnapshotCapableEventStore defines the interface needed for snapshot-aware operations.
// This extends the basic EventStore interface with snapshot management capabilities.
type SnapshotCapableEventStore interface {
	Query(ctx context.Context, filter eventstore.Filter) (
		eventstore.StorableEvents,
		eventstore.MaxSequenceNumberUint,
		error,
	)
	SaveSnapshot(ctx context.Context, snapshot eventstore.Snapshot) error
	LoadSnapshot(ctx context.Context, projectionType string, filter eventstore.Filter) (*eventstore.Snapshot, error)
}

// SnapshotAwareQueryHandler wraps a standard QueryHandler with snapshotting capabilities.
// It provides significant performance improvements by using incremental projection updates
// instead of rebuilding the entire projection from scratch on each query.
type SnapshotAwareQueryHandler struct {
	baseHandler      QueryHandler
	eventStore       SnapshotCapableEventStore
	metricsCollector shell.MetricsCollector
	tracingCollector shell.TracingCollector
	contextualLogger shell.ContextualLogger
	logger           shell.Logger
}

// NewSnapshotAwareQueryHandler creates a new snapshot-aware wrapper around the base query handler.
// The wrapper will attempt to use snapshots for performance optimization but will fall back
// to the base handler if snapshots are not available or incompatible.
// Observability components are automatically extracted from the base handler.
// Returns an error if the base handler's EventStore doesn't support snapshot operations.
func NewSnapshotAwareQueryHandler(baseHandler QueryHandler) (SnapshotAwareQueryHandler, error) {
	snapshotCapableStore, ok := baseHandler.eventStore.(SnapshotCapableEventStore)
	if !ok {
		return SnapshotAwareQueryHandler{}, ErrEventStoreNotSnapshotCapable
	}

	return SnapshotAwareQueryHandler{
		baseHandler:      baseHandler,
		eventStore:       snapshotCapableStore,
		metricsCollector: baseHandler.metricsCollector,
		tracingCollector: baseHandler.tracingCollector,
		contextualLogger: baseHandler.contextualLogger,
		logger:           baseHandler.logger,
	}, nil
}

// Handle executes the snapshot-aware query processing workflow.
// It attempts to load an existing snapshot and perform incremental updates,
// falling back to the base handler if snapshots are unavailable or incompatible.
func (h *SnapshotAwareQueryHandler) Handle(ctx context.Context, query Query) (RegisteredReaders, error) {
	// Start query handler instrumentation
	queryStart := time.Now()
	ctx, span := shell.StartQuerySpan(ctx, h.tracingCollector, queryType)
	shell.LogQueryStart(ctx, h.logger, h.contextualLogger, queryType)

	// Get the base filter for this query
	baseFilter := BuildEventFilter()

	// Snapshot Load phase
	snapshot, err := h.executeSnapshotLoad(ctx, baseFilter)
	if err != nil {
		return h.recordFallbackAndExecute(ctx, query, queryStart, span, shell.SnapshotReasonError)
	}

	// Fall back and then save the data as a snapshot
	if snapshot == nil {
		return h.recordFallbackAndExecute(ctx, query, queryStart, span, shell.SnapshotReasonMiss)
	}

	// Filter Reopening phase
	sequenceCapableFilter, err := h.executeFilterReopen(ctx, baseFilter)
	if err != nil {
		return h.recordFallbackAndExecute(ctx, query, queryStart, span, shell.SnapshotReasonFilterIncompatible)
	}

	// Incremental Query phase
	storableEvents, maxSeq, err := h.executeIncrementalQuery(ctx, sequenceCapableFilter, snapshot.SequenceNumber)
	if err != nil {
		return h.recordFallbackAndExecute(ctx, query, queryStart, span, shell.SnapshotReasonIncrementalQueryError)
	}

	// Unmarshal phase
	incrementalEvents, err := h.executeUnmarshal(ctx, storableEvents)
	if err != nil {
		return h.recordFallbackAndExecute(ctx, query, queryStart, span, shell.SnapshotReasonUnmarshalError)
	}

	// Snapshot Deserialization phase
	baseProjection, err := h.executeSnapshotDeserialization(ctx, snapshot)
	if err != nil {
		return h.recordFallbackAndExecute(ctx, query, queryStart, span, shell.SnapshotReasonDeserializeError)
	}

	// Determine the final sequence number (max of snapshot and incremental query)
	finalSequence := maxSeq
	if snapshot.SequenceNumber > finalSequence {
		finalSequence = snapshot.SequenceNumber
	}

	// Incremental Projection phase
	result := h.executeIncrementalProjection(ctx, incrementalEvents, query, baseProjection, finalSequence)

	// Save the updated snapshot with incremental changes
	h.saveUpdatedSnapshot(ctx, baseFilter, finalSequence, result)

	if h.contextualLogger != nil {
		h.contextualLogger.InfoContext(ctx, shell.LogMsgSnapshotHit,
			shell.LogAttrFromSequence, snapshot.SequenceNumber,
			shell.LogAttrToSequence, maxSeq,
			shell.LogAttrEventCount, len(incrementalEvents))
	} else if h.logger != nil {
		h.logger.Info(shell.LogMsgSnapshotHit,
			shell.LogAttrFromSequence, snapshot.SequenceNumber,
			shell.LogAttrToSequence, maxSeq,
			shell.LogAttrEventCount, len(incrementalEvents))
	}

	h.recordQuerySuccess(ctx, time.Since(queryStart), span, shell.SnapshotReasonHit)

	return result, nil
}

// BuildSnapshotType builds the projection type for the given queryType.
func (h *SnapshotAwareQueryHandler) BuildSnapshotType() string {
	return queryType
}

// Extracted phase execution methods for clean observability patterns

// executeSnapshotLoad handles the snapshot loading phase with proper observability.
func (h *SnapshotAwareQueryHandler) executeSnapshotLoad(
	ctx context.Context,
	filter eventstore.Filter,
) (*eventstore.Snapshot, error) {

	snapshotLoadStart := time.Now()
	snapshot, err := h.eventStore.LoadSnapshot(ctx, h.BuildSnapshotType(), filter)
	snapshotLoadDuration := time.Since(snapshotLoadStart)

	if err != nil {
		// Actual error (not just "not found")
		h.recordComponentTiming(ctx, shell.ComponentSnapshotLoad, shell.StatusError, snapshotLoadDuration)
		// Simple inline logging for snapshot load error
		if h.contextualLogger != nil {
			h.contextualLogger.ErrorContext(ctx, "snapshot load error", shell.LogAttrError, err.Error())
		} else if h.logger != nil {
			h.logger.Error("snapshot load error", shell.LogAttrError, err.Error())
		}
		return nil, err
	}

	if snapshot == nil {
		// Snapshot isn't found (normal case)
		h.recordComponentTiming(ctx, shell.ComponentSnapshotLoad, shell.StatusError, snapshotLoadDuration)
		// Simple inline logging for snapshot miss
		if h.contextualLogger != nil {
			h.contextualLogger.InfoContext(ctx, shell.LogMsgSnapshotMiss)
		} else if h.logger != nil {
			h.logger.Info(shell.LogMsgSnapshotMiss)
		}
		return nil, nil
	}

	// Snapshot found successfully
	h.recordComponentTiming(ctx, shell.ComponentSnapshotLoad, shell.StatusSuccess, snapshotLoadDuration)

	return snapshot, nil
}

// executeFilterReopen handles the filter reopening phase with proper observability.
func (h *SnapshotAwareQueryHandler) executeFilterReopen(
	ctx context.Context,
	baseFilter eventstore.Filter,
) (eventstore.SequenceFilteringCapable, error) {

	reopened := baseFilter.ReopenForSequenceFiltering()
	capable, ok := reopened.(eventstore.SequenceFilteringCapable)

	if !ok {
		var reason string
		if incompatible, ok := reopened.(eventstore.SequenceFilteringIncompatible); ok {
			reason = incompatible.CannotAddSequenceFiltering()
		} else {
			reason = "unknown reason"
		}
		if h.contextualLogger != nil {
			h.contextualLogger.InfoContext(ctx, shell.LogMsgSnapshotIncompatible, shell.LogAttrReason, reason)
		} else if h.logger != nil {
			h.logger.Info(shell.LogMsgSnapshotIncompatible, shell.LogAttrReason, reason)
		}
		return nil, ErrFilterNotSequenceCapable
	}

	return capable, nil
}

// executeIncrementalQuery handles the incremental query phase with proper observability.
func (h *SnapshotAwareQueryHandler) executeIncrementalQuery(
	ctx context.Context,
	capable eventstore.SequenceFilteringCapable,
	fromSequence uint,
) (eventstore.StorableEvents, eventstore.MaxSequenceNumberUint, error) {

	incrementalQueryStart := time.Now()
	incrementalFilter := capable.WithSequenceNumberHigherThan(fromSequence).Finalize()
	storableEvents, maxSeq, err := h.eventStore.Query(ctx, incrementalFilter)
	incrementalQueryDuration := time.Since(incrementalQueryStart)

	if err != nil {
		h.recordComponentTiming(ctx, shell.ComponentIncrementalQuery, shell.StatusError, incrementalQueryDuration)
		if h.contextualLogger != nil {
			h.contextualLogger.ErrorContext(ctx, shell.LogMsgIncrementalQueryError, shell.LogAttrError, err.Error())
		} else if h.logger != nil {
			h.logger.Error(shell.LogMsgIncrementalQueryError, shell.LogAttrError, err.Error())
		}
		return nil, 0, err
	}

	h.recordComponentTiming(ctx, shell.ComponentIncrementalQuery, shell.StatusSuccess, incrementalQueryDuration)

	return storableEvents, maxSeq, nil
}

// executeUnmarshal handles the event unmarshaling phase with proper observability.
func (h *SnapshotAwareQueryHandler) executeUnmarshal(
	ctx context.Context,
	storableEvents eventstore.StorableEvents,
) (core.DomainEvents, error) {

	unmarshalStart := time.Now()
	incrementalEvents, err := shell.DomainEventsFrom(storableEvents)
	unmarshalDuration := time.Since(unmarshalStart)

	if err != nil {
		h.recordComponentTiming(ctx, shell.ComponentUnmarshal, shell.StatusError, unmarshalDuration)
		if h.contextualLogger != nil {
			h.contextualLogger.ErrorContext(ctx, shell.LogMsgEventConversionError, shell.LogAttrError, err.Error())
		} else if h.logger != nil {
			h.logger.Error(shell.LogMsgEventConversionError, shell.LogAttrError, err.Error())
		}
		return nil, err
	}

	h.recordComponentTiming(ctx, shell.ComponentUnmarshal, shell.StatusSuccess, unmarshalDuration)

	return incrementalEvents, nil
}

// executeSnapshotDeserialization handles snapshot deserialization with proper observability.
func (h *SnapshotAwareQueryHandler) executeSnapshotDeserialization(
	ctx context.Context,
	snapshot *eventstore.Snapshot,
) (RegisteredReaders, error) {

	deserializationStart := time.Now()
	var baseProjection RegisteredReaders
	err := jsoniter.ConfigFastest.Unmarshal(snapshot.Data, &baseProjection)
	deserializationDuration := time.Since(deserializationStart)

	if err != nil {
		h.recordComponentTiming(ctx, shell.ComponentSnapshotDeserialize, shell.StatusError, deserializationDuration)
		if h.contextualLogger != nil {
			h.contextualLogger.ErrorContext(ctx, shell.LogMsgSnapshotDeserializationError, shell.LogAttrError, err.Error())
		} else if h.logger != nil {
			h.logger.Error(shell.LogMsgSnapshotDeserializationError, shell.LogAttrError, err.Error())
		}
		return RegisteredReaders{}, err
	}

	h.recordComponentTiming(ctx, shell.ComponentSnapshotDeserialize, shell.StatusSuccess, deserializationDuration)

	return baseProjection, nil
}

// executeIncrementalProjection handles incremental projection with proper observability.
func (h *SnapshotAwareQueryHandler) executeIncrementalProjection(
	ctx context.Context,
	incrementalEvents core.DomainEvents,
	query Query,
	baseProjection RegisteredReaders,
	maxSequence eventstore.MaxSequenceNumberUint,
) RegisteredReaders {

	incrementalProjectionStart := time.Now()
	result := Project(incrementalEvents, query, maxSequence, baseProjection)
	incrementalProjectionDuration := time.Since(incrementalProjectionStart)

	h.recordComponentTiming(ctx, shell.ComponentIncrementalProjection, shell.StatusSuccess, incrementalProjectionDuration)

	return result
}

// saveUpdatedSnapshot saves the updated projection as a snapshot.
// This is called synchronously to ensure reliable snapshot storage.
// Uses a background context with timeout to avoid cancellation issues.
func (h *SnapshotAwareQueryHandler) saveUpdatedSnapshot(
	parentCtx context.Context,
	filter eventstore.Filter,
	maxSequence eventstore.MaxSequenceNumberUint,
	projection RegisteredReaders,
) {
	// Create context with additional timeout for snapshot saving, inheriting cancellation
	ctx, cancel := context.WithTimeout(parentCtx, 30*time.Second)
	defer cancel()

	// Start instrumentation for the entire snapshot save operation
	snapshotSaveStart := time.Now()

	// Serialize projection to JSON
	data, err := jsoniter.ConfigFastest.Marshal(projection)
	if err != nil {
		h.recordSnapshotSaveError(ctx, "JSON serialization", err, snapshotSaveStart)
		return
	}

	// Build snapshot
	snapshot, err := eventstore.BuildSnapshot(
		h.BuildSnapshotType(),
		filter.Hash(),
		maxSequence,
		data,
	)
	if err != nil {
		h.recordSnapshotSaveError(ctx, "snapshot build", err, snapshotSaveStart)
		return
	}

	// Save snapshot
	if err := h.eventStore.SaveSnapshot(ctx, snapshot); err != nil {
		h.recordSnapshotSaveError(ctx, "snapshot save", err, snapshotSaveStart)
		return
	}

	// Record successful snapshot save
	h.recordComponentTiming(ctx, shell.ComponentSnapshotSave, shell.StatusSuccess, time.Since(snapshotSaveStart))

	if h.contextualLogger != nil {
		h.contextualLogger.InfoContext(ctx, shell.LogMsgSnapshotSaved, shell.LogAttrSequence, maxSequence)
	} else if h.logger != nil {
		h.logger.Info(shell.LogMsgSnapshotSaved, shell.LogAttrSequence, maxSequence)
	}
}

// Observability helper methods

// recordQuerySuccess records successful snapshot-aware query execution with observability.
func (h *SnapshotAwareQueryHandler) recordQuerySuccess(
	ctx context.Context,
	duration time.Duration,
	span shell.SpanContext,
	snapshotStatus string,
) {

	shell.RecordQueryMetrics(ctx, h.metricsCollector, queryType, shell.StatusSuccess, duration)
	shell.FinishQuerySpan(h.tracingCollector, span, shell.StatusSuccess, duration, nil)

	// Log success with snapshot status for better observability
	if h.contextualLogger != nil {
		h.contextualLogger.InfoContext(ctx, shell.LogMsgSnapshotQuerySuccess,
			shell.LogAttrQueryType, queryType,
			shell.LogAttrStatus, shell.StatusSuccess,
			shell.LogAttrDurationMS, duration.Milliseconds(),
			shell.LogAttrSnapshotStatus, snapshotStatus)
	} else if h.logger != nil {
		h.logger.Info(shell.LogMsgSnapshotQuerySuccess,
			shell.LogAttrQueryType, queryType,
			shell.LogAttrStatus, shell.StatusSuccess,
			shell.LogAttrDurationMS, duration.Milliseconds(),
			shell.LogAttrSnapshotStatus, snapshotStatus)
	}
}

// recordFallbackAndExecute records the fallback scenario and delegates to base handler.
func (h *SnapshotAwareQueryHandler) recordFallbackAndExecute(
	ctx context.Context,
	query Query,
	queryStart time.Time,
	span shell.SpanContext,
	fallbackReason string,
) (RegisteredReaders, error) {

	duration := time.Since(queryStart)

	// Record the fallback as a successful operation (since base handler will handle it)
	shell.RecordQueryMetrics(ctx, h.metricsCollector, queryType, shell.StatusSuccess, duration)
	shell.FinishQuerySpan(h.tracingCollector, span, shell.StatusSuccess, duration, nil)

	if h.contextualLogger != nil {
		h.contextualLogger.InfoContext(ctx, shell.LogMsgSnapshotFallback,
			shell.LogAttrReason, fallbackReason,
			shell.LogAttrDurationMS, duration.Milliseconds())
	} else if h.logger != nil {
		h.logger.Info(shell.LogMsgSnapshotFallback,
			shell.LogAttrReason, fallbackReason,
			shell.LogAttrDurationMS, duration.Milliseconds())
	}

	// Delegate to base handler
	result, err := h.baseHandler.Handle(ctx, query)
	if err != nil {
		return result, err
	}

	// For snapshot miss, save the result as an initial snapshot for future queries
	if fallbackReason == shell.SnapshotReasonMiss {
		baseFilter := BuildEventFilter()
		// Use the sequence number from the result (no double query needed!)
		h.saveUpdatedSnapshot(ctx, baseFilter, result.SequenceNumber, result)
	}

	return result, nil
}

// recordComponentTiming records component-level timing metrics for snapshot operations.
func (h *SnapshotAwareQueryHandler) recordComponentTiming(
	ctx context.Context,
	component string,
	status string,
	duration time.Duration,
) {

	shell.RecordQueryComponentDuration(ctx, h.metricsCollector, queryType, component, status, duration)
}

// recordSnapshotSaveError handles error recording and logging for snapshot save operations.
func (h *SnapshotAwareQueryHandler) recordSnapshotSaveError(
	ctx context.Context,
	operation string,
	err error,
	startTime time.Time,
) {

	h.recordComponentTiming(ctx, shell.ComponentSnapshotSave, shell.StatusError, time.Since(startTime))

	if h.contextualLogger != nil {
		h.contextualLogger.ErrorContext(ctx, shell.LogMsgSnapshotSaveError,
			shell.LogAttrOperation, operation,
			shell.LogAttrError, err.Error())
	} else if h.logger != nil {
		h.logger.Error(shell.LogMsgSnapshotSaveError,
			shell.LogAttrOperation, operation,
			shell.LogAttrError, err.Error())
	}
}
