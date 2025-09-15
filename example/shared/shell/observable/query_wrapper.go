package observable

import (
	"context"
	"time"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell"
)

// QueryWrapper provides comprehensive observability instrumentation for any query handler.
// It wraps a core query handler and adds metrics, tracing, and logging.
// The wrapper handles all infrastructure concerns while delegating business logic to the wrapped handler.
// This follows the same composition pattern as CommandWrapper for consistency.
type QueryWrapper[Q shell.Query, R shell.QueryResult] struct {
	coreHandler      shell.QueryHandler[Q, R]
	queryType        string
	metricsCollector shell.MetricsCollector
	tracingCollector shell.TracingCollector
	contextualLogger shell.ContextualLogger
	logger           shell.Logger
}

// NewQueryWrapper creates a new observable wrapper around the core query handler.
// The wrapper will instrument the handler with comprehensive observability while delegating
// all business logic to the wrapped handler.
func NewQueryWrapper[Q shell.Query, R shell.QueryResult](
	coreHandler shell.QueryHandler[Q, R],
	opts ...QueryOption[Q, R],
) (*QueryWrapper[Q, R], error) {
	// Extract query type from a zero-value instance
	var zeroQuery Q
	queryType := zeroQuery.QueryType()

	wrapper := &QueryWrapper[Q, R]{
		coreHandler: coreHandler,
		queryType:   queryType,
	}

	// Apply options
	for _, opt := range opts {
		if err := opt(wrapper); err != nil {
			return nil, err
		}
	}

	return wrapper, nil
}

// Handle executes the complete query processing workflow with comprehensive observability.
// It instruments the operation with metrics, tracing, and logging while delegating the actual
// business logic to the wrapped core handler.
//
// The wrapper provides pure observability decoration, extracting all the necessary info from timing and errors.
func (w *QueryWrapper[Q, R]) Handle(ctx context.Context, query Q) (R, error) {
	// Start query handler instrumentation
	queryStart := time.Now()
	ctx, span := shell.StartQuerySpan(ctx, w.tracingCollector, w.queryType)
	shell.LogQueryStart(ctx, w.logger, w.contextualLogger, w.queryType)

	// Delegate to the wrapped handler
	result, err := w.coreHandler.Handle(ctx, query)

	duration := time.Since(queryStart)
	if err != nil {
		w.recordQueryError(ctx, err, duration, span)
		return result, err
	}

	w.recordQuerySuccess(ctx, duration, span)
	return result, nil
}

// QueryOption defines a functional option for configuring QueryWrapper.
type QueryOption[Q shell.Query, R shell.QueryResult] func(*QueryWrapper[Q, R]) error

// WithQueryMetrics sets the metrics collector for the QueryWrapper.
func WithQueryMetrics[Q shell.Query, R shell.QueryResult](collector shell.MetricsCollector) QueryOption[Q, R] {
	return func(w *QueryWrapper[Q, R]) error {
		w.metricsCollector = collector
		return nil
	}
}

// WithQueryTracing sets the tracing collector for the QueryWrapper.
func WithQueryTracing[Q shell.Query, R shell.QueryResult](collector shell.TracingCollector) QueryOption[Q, R] {
	return func(w *QueryWrapper[Q, R]) error {
		w.tracingCollector = collector
		return nil
	}
}

// WithQueryContextualLogging sets the contextual logger for the QueryWrapper.
func WithQueryContextualLogging[Q shell.Query, R shell.QueryResult](logger shell.ContextualLogger) QueryOption[Q, R] {
	return func(w *QueryWrapper[Q, R]) error {
		w.contextualLogger = logger
		return nil
	}
}

// WithQueryLogging sets the basic logger for the QueryWrapper.
func WithQueryLogging[Q shell.Query, R shell.QueryResult](logger shell.Logger) QueryOption[Q, R] {
	return func(w *QueryWrapper[Q, R]) error {
		w.logger = logger
		return nil
	}
}

/*** Observability helper methods ***/

// recordQuerySuccess records successful query execution with observability.
func (w *QueryWrapper[Q, R]) recordQuerySuccess(ctx context.Context, duration time.Duration, span shell.SpanContext) {
	shell.RecordQueryMetrics(ctx, w.metricsCollector, w.queryType, shell.StatusSuccess, duration, "")
	shell.FinishQuerySpan(w.tracingCollector, span, shell.StatusSuccess, duration, nil)
	shell.LogQuerySuccess(ctx, w.logger, w.contextualLogger, w.queryType, shell.StatusSuccess, duration)
}

// recordQueryError records failed query execution with observability.
func (w *QueryWrapper[Q, R]) recordQueryError(ctx context.Context, err error, duration time.Duration, span shell.SpanContext) {
	if shell.IsCancellationError(err) {
		w.recordQueryCancelled(ctx, err, duration, span)
		return
	}

	if shell.IsTimeoutError(err) {
		w.recordQueryTimeout(ctx, err, duration, span)
		return
	}

	shell.RecordQueryMetrics(ctx, w.metricsCollector, w.queryType, shell.StatusError, duration, "")
	shell.FinishQuerySpan(w.tracingCollector, span, shell.StatusError, duration, err)
	shell.LogQueryError(ctx, w.logger, w.contextualLogger, w.queryType, err)
}

// recordQueryCancelled records canceled query execution with observability.
func (w *QueryWrapper[Q, R]) recordQueryCancelled(ctx context.Context, err error, duration time.Duration, span shell.SpanContext) {
	shell.RecordQueryMetrics(ctx, w.metricsCollector, w.queryType, shell.StatusCanceled, duration, "")
	shell.FinishQuerySpan(w.tracingCollector, span, shell.StatusCanceled, duration, err)
	shell.LogQueryError(ctx, w.logger, w.contextualLogger, w.queryType, err)
}

// recordQueryTimeout records timeout query execution with observability.
func (w *QueryWrapper[Q, R]) recordQueryTimeout(ctx context.Context, err error, duration time.Duration, span shell.SpanContext) {
	shell.RecordQueryMetrics(ctx, w.metricsCollector, w.queryType, shell.StatusTimeout, duration, "")
	shell.FinishQuerySpan(w.tracingCollector, span, shell.StatusTimeout, duration, err)
	shell.LogQueryError(ctx, w.logger, w.contextualLogger, w.queryType, err)
}
