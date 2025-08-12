package postgresengine

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/postgres" // driver import
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jmoiron/sqlx"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine/internal/adapters"
)

const (
	// Core database configuration.
	defaultEventTableName = "events"
	dialectPostgres       = "postgres"

	// Database schema column names.
	colEventType      = "event_type"
	colOccurredAt     = "occurred_at"
	colPayload        = "payload"
	colMetadata       = "metadata"
	colSequenceNumber = "sequence_number"

	// SQL query building constants.
	cteContext    = "context"
	cteVals       = "vals"
	aliasMaxSeq   = "max_seq"
	castText      = "?::text"
	castTimestamp = "?::timestamp with time zone"
	castJsonb     = "?::jsonb"
)

type (
	sqlQueryString    = string
	rowsAffectedInt64 = int64
	queryDuration     = time.Duration
)

// EventStore represents a storage mechanism for appending and querying events in an event sourcing implementation.
// It leverages a database adapter and supports customizable logging, metricsCollector collection, tracing, and event table configuration.
type EventStore struct {
	db               adapters.DBAdapter
	eventTableName   string
	logger           Logger
	metricsCollector MetricsCollector
	tracingCollector TracingCollector
	contextualLogger ContextualLogger
	builderPool      *sync.Pool
}

type queryResultRow struct {
	eventType         string
	payload           []byte
	metadata          []byte
	occurredAt        time.Time
	maxSequenceNumber eventstore.MaxSequenceNumberUint
}

// NewEventStoreFromPGXPool creates a new EventStore using a pgx.Pool with optional configuration.
func NewEventStoreFromPGXPool(db *pgxpool.Pool, options ...Option) (*EventStore, error) {
	if db == nil {
		return nil, eventstore.ErrNilDatabaseConnection
	}

	es := &EventStore{
		db:             adapters.NewPGXAdapter(db),
		eventTableName: defaultEventTableName,
		builderPool: &sync.Pool{
			New: func() interface{} {
				dialect := goqu.Dialect(dialectPostgres)
				return &dialect
			},
		},
	}

	for _, option := range options {
		if err := option(es); err != nil {
			return nil, err
		}
	}

	return es, nil
}

// NewEventStoreFromPGXPoolAndReplica creates a new EventStore using a primary pgx.Pool
// and a replica pgx.Pool with optional configuration.
func NewEventStoreFromPGXPoolAndReplica(db *pgxpool.Pool, replica *pgxpool.Pool, options ...Option) (*EventStore, error) {
	if db == nil {
		return nil, eventstore.ErrNilDatabaseConnection
	}

	es := &EventStore{
		db:             adapters.NewPGXAdapterWithReplica(db, replica),
		eventTableName: defaultEventTableName,
		builderPool: &sync.Pool{
			New: func() interface{} {
				dialect := goqu.Dialect(dialectPostgres)
				return &dialect
			},
		},
	}

	for _, option := range options {
		if err := option(es); err != nil {
			return nil, err
		}
	}

	return es, nil
}

// NewEventStoreFromSQLDB creates a new EventStore using a sql.DB with optional configuration.
func NewEventStoreFromSQLDB(db *sql.DB, options ...Option) (*EventStore, error) {
	if db == nil {
		return nil, eventstore.ErrNilDatabaseConnection
	}

	es := &EventStore{
		db:             adapters.NewSQLAdapter(db),
		eventTableName: defaultEventTableName,
		builderPool: &sync.Pool{
			New: func() interface{} {
				dialect := goqu.Dialect(dialectPostgres)
				return &dialect
			},
		},
	}

	for _, option := range options {
		if err := option(es); err != nil {
			return nil, err
		}
	}

	return es, nil
}

// NewEventStoreFromSQLDBAndReplica creates a new EventStore using a primary sql.DB
// and a replica sql.DB with optional configuration.
func NewEventStoreFromSQLDBAndReplica(db *sql.DB, replica *sql.DB, options ...Option) (*EventStore, error) {
	if db == nil {
		return nil, eventstore.ErrNilDatabaseConnection
	}

	es := &EventStore{
		db:             adapters.NewSQLAdapterWithReplica(db, replica),
		eventTableName: defaultEventTableName,
		builderPool: &sync.Pool{
			New: func() interface{} {
				dialect := goqu.Dialect(dialectPostgres)
				return &dialect
			},
		},
	}

	for _, option := range options {
		if err := option(es); err != nil {
			return nil, err
		}
	}

	return es, nil
}

// NewEventStoreFromSQLX creates a new EventStore using a sqlx.DB with optional configuration.
func NewEventStoreFromSQLX(db *sqlx.DB, options ...Option) (*EventStore, error) {
	if db == nil {
		return nil, eventstore.ErrNilDatabaseConnection
	}

	es := &EventStore{
		db:             adapters.NewSQLXAdapter(db),
		eventTableName: defaultEventTableName,
		builderPool: &sync.Pool{
			New: func() interface{} {
				dialect := goqu.Dialect(dialectPostgres)
				return &dialect
			},
		},
	}

	for _, option := range options {
		if err := option(es); err != nil {
			return nil, err
		}
	}

	return es, nil
}

// NewEventStoreFromSQLXAndReplica creates a new EventStore using a primary sqlx.DB
// and a replica sqlx.DB with optional configuration.
func NewEventStoreFromSQLXAndReplica(db *sqlx.DB, replica *sqlx.DB, options ...Option) (*EventStore, error) {
	if db == nil {
		return nil, eventstore.ErrNilDatabaseConnection
	}

	es := &EventStore{
		db:             adapters.NewSQLXAdapterWithReplica(db, replica),
		eventTableName: defaultEventTableName,
		builderPool: &sync.Pool{
			New: func() interface{} {
				dialect := goqu.Dialect(dialectPostgres)
				return &dialect
			},
		},
	}

	for _, option := range options {
		if err := option(es); err != nil {
			return nil, err
		}
	}

	return es, nil
}

// Query retrieves events from the Postgres event store based on the provided eventstore.Filter criteria
// and returns them as eventstore.StorableEvents
// as well as the MaxSequenceNumberUint for this "dynamic event stream" at the time of the query.
func (es *EventStore) Query(ctx context.Context, filter eventstore.Filter) (
	eventstore.StorableEvents,
	eventstore.MaxSequenceNumberUint,
	error,
) {
	var empty eventstore.StorableEvents

	// Setup instrumentation
	inst, ctx := es.setupQueryInstrumentation(ctx)

	// Core business logic with component timing
	sqlQuery, err := es.executeQueryBuildPhase(ctx, filter, inst)
	if err != nil {
		return empty, 0, err
	}

	rows, sqlDuration, err := es.executeQueryExecutionPhase(ctx, sqlQuery, inst)
	if err != nil {
		return empty, 0, err
	}
	defer es.closeRows(rows)

	eventStream, maxSeq, err := es.executeQueryProcessingPhase(rows, inst)
	if err != nil {
		return empty, 0, err
	}

	// Success completion
	es.completeQuerySuccess(ctx, eventStream, maxSeq, sqlDuration, inst)

	return eventStream, maxSeq, nil
}

// executeQueryBuildPhase builds the SQL query with component timing and error handling.
func (es *EventStore) executeQueryBuildPhase(
	ctx context.Context,
	filter eventstore.Filter,
	inst queryInstrumentation,
) (string, error) {
	// Phase 2: Component-level timing - Query Building
	queryBuildStart := time.Now()
	sqlQuery, buildQueryErr := es.buildSelectQuery(filter)
	queryBuildDuration := time.Since(queryBuildStart)

	if buildQueryErr != nil {
		methodDuration := time.Since(inst.methodStart)
		es.logError(logMsgBuildSelectQueryFailed, buildQueryErr)
		es.logErrorContext(ctx, logMsgBuildSelectQueryFailed, buildQueryErr)
		inst.metrics.recordError(errorTypeBuildQuery, methodDuration)
		inst.tracer.finishError(errorTypeBuildQuery, methodDuration)

		return "", buildQueryErr
	}

	// Record query build component timing
	inst.metrics.recordComponentSuccess(componentQueryBuild, queryBuildDuration)

	return sqlQuery, nil
}

// executeQueryExecutionPhase executes the SQL query with timing and error handling.
func (es *EventStore) executeQueryExecutionPhase(
	ctx context.Context,
	sqlQuery string,
	inst queryInstrumentation,
) (adapters.DBRows, time.Duration, error) {
	// Phase 2: Component-level timing - SQL Execution
	rows, sqlDuration, queryErr := es.executeQuery(ctx, sqlQuery)
	if queryErr != nil {
		methodDuration := time.Since(inst.methodStart)
		switch {
		case es.isCancellationError(queryErr):
			inst.metrics.recordCanceled(methodDuration)
			inst.tracer.finishCancelled(methodDuration)
		case es.isTimeoutError(queryErr):
			inst.metrics.recordTimeout(methodDuration)
			inst.tracer.finishTimeout(methodDuration)
		default:
			inst.metrics.recordError(errorTypeDatabaseQuery, methodDuration)
			inst.tracer.finishError(errorTypeDatabaseQuery, methodDuration)
		}

		return nil, sqlDuration, queryErr
	}

	// Record SQL execution component timing
	inst.metrics.recordComponentSuccess(componentSQLExecution, sqlDuration)

	return rows, sqlDuration, nil
}

// executeQueryProcessingPhase processes query results with timing and error handling.
func (es *EventStore) executeQueryProcessingPhase(
	rows adapters.DBRows,
	inst queryInstrumentation,
) (eventstore.StorableEvents, eventstore.MaxSequenceNumberUint, error) {
	var empty eventstore.StorableEvents

	// Phase 2: Component-level timing - Result Processing
	resultProcessStart := time.Now()
	eventStream, maxSequenceNumber, scanErr := es.processQueryResults(rows, inst.metrics)
	resultProcessDuration := time.Since(resultProcessStart)

	if scanErr != nil {
		methodDuration := time.Since(inst.methodStart)
		inst.tracer.finishError(errorTypeScanResults, methodDuration)
		return empty, 0, scanErr
	}

	// Record result processing component timing
	inst.metrics.recordComponentSuccess(componentResultProcessing, resultProcessDuration)

	return eventStream, maxSequenceNumber, nil
}

func (es *EventStore) buildSelectQuery(filter eventstore.Filter) (sqlQueryString, error) {
	builder := es.getBuilder()
	defer es.putBuilder(builder)

	selectStmt := builder.
		From(es.eventTableName).
		Select(colEventType, colOccurredAt, colPayload, colMetadata, colSequenceNumber).
		Order(goqu.I(colSequenceNumber).Asc())

	selectStmt = es.addWhereClause(filter, selectStmt)

	sqlQuery, _, toSQLErr := selectStmt.ToSQL()
	if toSQLErr != nil {
		return "", errors.Join(eventstore.ErrBuildingQueryFailed, toSQLErr)
	}

	return sqlQuery, nil
}

// executeQuery executes the SQL query and returns rows with timing information.
func (es *EventStore) executeQuery(ctx context.Context, sqlQuery string) (
	adapters.DBRows,
	time.Duration,
	error,
) {

	start := time.Now()
	rows, queryErr := es.db.Query(ctx, sqlQuery)
	duration := time.Since(start)
	es.logQueryWithDuration(sqlQuery, logActionQuery, duration)
	es.logQueryWithDurationContext(ctx, sqlQuery, logActionQuery, duration)

	if queryErr != nil {
		es.logError(logMsgDBQueryFailed, queryErr, logAttrQuery, sqlQuery)
		es.logErrorContext(ctx, logMsgDBQueryFailed, queryErr, logAttrQuery, sqlQuery)
		return nil, duration, errors.Join(eventstore.ErrQueryingEventsFailed, queryErr)
	}

	return rows, duration, nil
}

// closeRows safely closes database rows and logs any errors.
func (es *EventStore) closeRows(rows adapters.DBRows) {
	if closeErr := rows.Close(); closeErr != nil {
		if es.logger != nil {
			es.logger.Warn(logMsgCloseRowsFailed, logAttrError, closeErr.Error())
		}
	}
}

// processQueryResults processes database rows and converts them to domain events.
func (es *EventStore) processQueryResults(rows adapters.DBRows, metrics *queryMetricsObserver) (
	eventstore.StorableEvents,
	eventstore.MaxSequenceNumberUint,
	error,
) {

	var empty eventstore.StorableEvents
	result := queryResultRow{}
	eventStream := make(eventstore.StorableEvents, 0)
	maxSequenceNumber := eventstore.MaxSequenceNumberUint(0)

	for rows.Next() {
		rowScanErr := rows.Scan(&result.eventType, &result.occurredAt, &result.payload, &result.metadata, &result.maxSequenceNumber)
		if rowScanErr != nil {
			es.logError(logMsgScanRowFailed, rowScanErr)
			metrics.recordError(errorTypeRowScan, 0)

			return empty, 0, errors.Join(eventstore.ErrScanningDBRowFailed, rowScanErr)
		}

		event, buildStorableErr := eventstore.BuildStorableEvent(result.eventType, result.occurredAt, result.payload, result.metadata)
		if buildStorableErr != nil {
			es.logError(logMsgBuildStorableEventFailed, buildStorableErr, logAttrEventType, result.eventType)
			metrics.recordError(errorTypeBuildStorableEvent, 0)

			return empty, 0, errors.Join(eventstore.ErrBuildingStorableEventFailed, buildStorableErr)
		}

		eventStream = append(eventStream, event)
		maxSequenceNumber = result.maxSequenceNumber
	}

	return eventStream, maxSequenceNumber, nil
}

// Append attempts to append one or multiple eventstore.StorableEvent(s) onto the Postgres event store respecting concurrency constraints
// for this "dynamic event stream" based on the provided eventstore.Filter criteria and the expected MaxSequenceNumberUint.
//
// The provided eventstore.Filter criteria should be the same as the ones used for the Query before making the business decisions.
//
// The insert query to append multiple events atomically is heavier than the one built to append a single event.
// In event-sourced applications, one command/request should typically only produce one event.
// Only supply multiple events if you are sure that you need to append multiple events at once!
func (es *EventStore) Append(
	ctx context.Context,
	filter eventstore.Filter,
	expectedMaxSequenceNumber eventstore.MaxSequenceNumberUint,
	storableEvents ...eventstore.StorableEvent,
) error {
	if len(storableEvents) == 0 {
		return nil // nothing to do
	}

	// Setup instrumentation
	inst, ctx := es.setupAppendInstrumentation(ctx, storableEvents, expectedMaxSequenceNumber)

	// Core business logic with component timing
	sqlQuery, err := es.executeAppendBuildPhase(ctx, storableEvents, filter, expectedMaxSequenceNumber, inst)
	if err != nil {
		return err
	}

	rowsAffected, sqlDuration, err := es.executeAppendExecutionPhase(ctx, sqlQuery, inst)
	if err != nil {
		return err
	}

	err = es.executeAppendValidationPhase(ctx, rowsAffected, len(storableEvents), expectedMaxSequenceNumber, inst)
	if err != nil {
		return err
	}

	// Success completion
	es.completeAppendSuccess(ctx, len(storableEvents), rowsAffected, sqlDuration, inst)

	return nil
}

// executeAppendBuildPhase builds the SQL query for append with component timing and error handling.
func (es *EventStore) executeAppendBuildPhase(
	ctx context.Context,
	storableEvents eventstore.StorableEvents,
	filter eventstore.Filter,
	expectedMaxSequenceNumber eventstore.MaxSequenceNumberUint,
	inst appendInstrumentation,
) (string, error) {
	// Phase 2: Component-level timing - Query Building
	queryBuildStart := time.Now()
	sqlQuery, buildQueryErr := es.buildAppendQuery(ctx, storableEvents, filter, expectedMaxSequenceNumber, inst.metrics)
	queryBuildDuration := time.Since(queryBuildStart)

	if buildQueryErr != nil {
		methodDuration := time.Since(inst.methodStart)
		inst.tracer.finishErrorWithAttrs(errorTypeBuildQuery, map[string]string{
			spanAttrDurationMS: fmt.Sprintf("%.2f", es.toMilliseconds(methodDuration)),
		})
		return "", buildQueryErr
	}

	// Record query build component timing
	inst.metrics.recordComponentSuccess(componentQueryBuild, queryBuildDuration)

	return sqlQuery, nil
}

// executeAppendExecutionPhase executes the SQL query for append with timing and error handling.
func (es *EventStore) executeAppendExecutionPhase(
	ctx context.Context,
	sqlQuery string,
	inst appendInstrumentation,
) (int64, time.Duration, error) {
	// Phase 2: Component-level timing - SQL Execution
	rowsAffected, sqlDuration, execErr := es.executeAppendQuery(ctx, sqlQuery, inst.metrics)
	if execErr != nil {
		methodDuration := time.Since(inst.methodStart)
		switch {
		case es.isCancellationError(execErr):
			inst.metrics.recordCanceled(methodDuration)
			inst.tracer.finishCancelled(methodDuration)
		case es.isTimeoutError(execErr):
			inst.metrics.recordTimeout(methodDuration)
			inst.tracer.finishTimeout(methodDuration)
		default:
			inst.tracer.finishError(errorTypeDatabaseExec, methodDuration)
		}

		return 0, sqlDuration, execErr
	}

	// Record SQL execution component timing
	inst.metrics.recordComponentSuccess(componentSQLExecution, sqlDuration)

	return rowsAffected, sqlDuration, nil
}

// executeAppendValidationPhase validates the append operation results with error handling.
func (es *EventStore) executeAppendValidationPhase(
	ctx context.Context,
	rowsAffected int64,
	eventCount int,
	expectedMaxSequenceNumber eventstore.MaxSequenceNumberUint,
	inst appendInstrumentation,
) error {
	// Validate the result from the append operation (no timing needed - this is instant)
	validationErr := es.validateAppendResult(ctx, rowsAffected, eventCount, expectedMaxSequenceNumber, inst.metrics)
	if validationErr != nil {
		methodDuration := time.Since(inst.methodStart)
		attrs := map[string]string{
			spanAttrRowsAffected: fmt.Sprintf("%d", rowsAffected),
			spanAttrDurationMS:   fmt.Sprintf("%.2f", es.toMilliseconds(methodDuration)),
		}
		inst.tracer.finishErrorWithAttrs(errorTypeConcurrencyConflict, attrs)

		return validationErr
	}

	return nil
}

// buildAppendQuery builds the appropriate SQL query for single or multiple events.
func (es *EventStore) buildAppendQuery(
	ctx context.Context,
	storableEvents eventstore.StorableEvents,
	filter eventstore.Filter,
	expectedMaxSequenceNumber eventstore.MaxSequenceNumberUint,
	metrics *appendMetricsObserver,
) (sqlQueryString, error) {

	var sqlQuery sqlQueryString
	var buildQueryErr error

	switch len(storableEvents) {
	case 0:
		return "", eventstore.ErrCantBuildQueryForZeroEvents

	case 1:
		sqlQuery, buildQueryErr = es.buildInsertQueryForSingleEvent(
			storableEvents[0],
			filter,
			expectedMaxSequenceNumber,
		)

	default:
		sqlQuery, buildQueryErr = es.buildInsertQueryForMultipleEvents(
			storableEvents,
			filter,
			expectedMaxSequenceNumber,
		)
	}

	if buildQueryErr != nil {
		es.logError(logMsgBuildInsertQueryFailed, buildQueryErr, logAttrEventCount, len(storableEvents))
		es.logErrorContext(ctx, logMsgBuildInsertQueryFailed, buildQueryErr, logAttrEventCount, len(storableEvents))
		metrics.recordError(errorTypeBuildQuery, 0)

		return "", buildQueryErr
	}

	return sqlQuery, nil
}

func (es *EventStore) buildInsertQueryForSingleEvent(
	event eventstore.StorableEvent,
	filter eventstore.Filter,
	expectedMaxSequenceNumber eventstore.MaxSequenceNumberUint,
) (sqlQueryString, error) {

	builder := es.getBuilder()
	defer es.putBuilder(builder)

	// Define the subquery for the CTE
	cteStmt := builder.
		From(es.eventTableName).
		Select(goqu.MAX(colSequenceNumber).As(aliasMaxSeq))

	cteStmt = es.addWhereClause(filter, cteStmt)

	// Define the SELECT for the INSERT
	selectStmt := builder.
		From(cteContext).
		Select(goqu.V(event.EventType), goqu.V(event.OccurredAt), goqu.V(event.PayloadJSON), goqu.V(event.MetadataJSON)).
		Where(goqu.COALESCE(goqu.C(aliasMaxSeq), 0).Eq(goqu.V(expectedMaxSequenceNumber)))

	// Finalize the full INSERT query
	insertStmt := builder.
		Insert(es.eventTableName).
		Cols(colEventType, colOccurredAt, colPayload, colMetadata).
		FromQuery(selectStmt).
		With(cteContext, cteStmt)

	sqlQuery, _, toSQLErr := insertStmt.ToSQL()
	if toSQLErr != nil {
		es.logError(logMsgSingleEventSQLFailed, toSQLErr, logAttrEventType, event.EventType)
		es.recordErrorMetrics(operationAppend, errorTypeBuildSingleEventSQL)

		return "", errors.Join(eventstore.ErrBuildingQueryFailed, toSQLErr)
	}

	return sqlQuery, nil
}

func (es *EventStore) buildInsertQueryForMultipleEvents(
	events []eventstore.StorableEvent,
	filter eventstore.Filter,
	expectedMaxSequenceNumber eventstore.MaxSequenceNumberUint,
) (sqlQueryString, error) {

	builder := es.getBuilder()
	defer es.putBuilder(builder)

	// Define the subquery for the CTE
	cteStmt := builder.
		From(es.eventTableName).
		Select(goqu.MAX(colSequenceNumber).As(aliasMaxSeq))

	cteStmt = es.addWhereClause(filter, cteStmt)

	// Create individual SELECT statements for each event
	unionStatements := make([]*goqu.SelectDataset, len(events))
	for i, event := range events {
		unionStatements[i] = builder.
			Select(
				goqu.L(castText, event.EventType).As(colEventType),
				goqu.L(castTimestamp, event.OccurredAt).As(colOccurredAt),
				goqu.L(castJsonb, event.PayloadJSON).As(colPayload),
				goqu.L(castJsonb, event.MetadataJSON).As(colMetadata),
			)
	}

	// Combine all SELECT statements with UNION ALL
	valuesStmt := unionStatements[0]
	for i := 1; i < len(unionStatements); i++ {
		valuesStmt = valuesStmt.UnionAll(unionStatements[i])
	}

	// Finalize the full INSERT query
	valsEventType := fmt.Sprintf("%s.%s", cteVals, colEventType)
	valsOccurredAt := fmt.Sprintf("%s.%s", cteVals, colOccurredAt)
	valsPayload := fmt.Sprintf("%s.%s", cteVals, colPayload)
	valsMetadata := fmt.Sprintf("%s.%s", cteVals, colMetadata)

	insertStmt := builder.
		Insert(es.eventTableName).
		Cols(colEventType, colOccurredAt, colPayload, colMetadata).
		With(cteContext, cteStmt).
		With(cteVals, valuesStmt).
		FromQuery(
			builder.From(cteContext, cteVals).
				Select(valsEventType, valsOccurredAt, valsPayload, valsMetadata).
				Where(goqu.COALESCE(goqu.C(aliasMaxSeq), 0).Eq(goqu.V(expectedMaxSequenceNumber))),
		)

	sqlQuery, _, toSQLErr := insertStmt.ToSQL()
	if toSQLErr != nil {
		es.logError(logMsgMultiEventSQLFailed, toSQLErr, logAttrEventCount, len(events))
		es.recordErrorMetrics(operationAppend, errorTypeBuildMultiEventSQL)

		return "", errors.Join(eventstore.ErrBuildingQueryFailed, toSQLErr)
	}

	return sqlQuery, nil
}

// executeAppendQuery executes the SQL append query and returns rows affected and duration.
func (es *EventStore) executeAppendQuery(ctx context.Context, sqlQuery string, metrics *appendMetricsObserver) (
	rowsAffectedInt64,
	queryDuration,
	error,
) {

	start := time.Now()
	tag, execErr := es.db.Exec(ctx, sqlQuery)
	duration := time.Since(start)
	es.logQueryWithDuration(sqlQuery, logActionAppend, duration)
	es.logQueryWithDurationContext(ctx, sqlQuery, logActionAppend, duration)

	if execErr != nil {
		es.logError(logMsgDBExecFailed, execErr, logAttrQuery, sqlQuery)
		es.logErrorContext(ctx, logMsgDBExecFailed, execErr, logAttrQuery, sqlQuery)
		metrics.recordError(errorTypeDatabaseExec, duration)

		return 0, duration, errors.Join(eventstore.ErrAppendingEventFailed, execErr)
	}

	rowsAffected, rowsAffectedErr := tag.RowsAffected()
	if rowsAffectedErr != nil {
		es.logError(logMsgRowsAffectedFailed, rowsAffectedErr)
		es.logErrorContext(ctx, logMsgRowsAffectedFailed, rowsAffectedErr)
		metrics.recordError(errorTypeRowsAffected, 0)

		return 0, duration, errors.Join(eventstore.ErrGettingRowsAffectedFailed, rowsAffectedErr)
	}

	return rowsAffected, duration, nil
}

// validateAppendResult checks if the append operation was successful and detects concurrency conflicts.
func (es *EventStore) validateAppendResult(
	ctx context.Context,
	rowsAffected int64,
	expectedEventCount int,
	expectedMaxSequenceNumber eventstore.MaxSequenceNumberUint,
	metrics *appendMetricsObserver,
) error {

	if rowsAffected < int64(expectedEventCount) {
		es.logOperation(
			logMsgConcurrencyConflict,
			logAttrExpectedEvents, expectedEventCount,
			logAttrRowsAffected, rowsAffected,
			logAttrExpectedSequence, expectedMaxSequenceNumber,
		)
		es.logOperationContext(ctx, logMsgConcurrencyConflict,
			logAttrExpectedEvents, expectedEventCount,
			logAttrRowsAffected, rowsAffected,
			logAttrExpectedSequence, expectedMaxSequenceNumber,
		)

		metrics.recordConcurrencyConflict()

		return eventstore.ErrConcurrencyConflict
	}

	return nil
}

func (es *EventStore) addWhereClause(filter eventstore.Filter, selectStmt *goqu.SelectDataset) *goqu.SelectDataset {
	itemsExpressions := make([]goqu.Expression, 0)

	for _, item := range filter.Items() {
		eventTypeExpressions := make([]goqu.Expression, 0)
		predicateExpressions := make([]goqu.Expression, 0)

		for _, eventType := range item.EventTypes() {
			eventTypeExpressions = append(
				eventTypeExpressions,
				goqu.Ex{colEventType: eventType},
			)
		}

		// eventTypes must always be filtered with OR ;-)
		eventTypesExpressionList := goqu.Or(eventTypeExpressions...)

		for _, predicate := range item.Predicates() {
			predicateExpressions = append(
				predicateExpressions,
				goqu.L(fmt.Sprintf(`%s @> '{"%s": "%s"}'`, colPayload, predicate.Key(), predicate.Val())),
			)
		}

		var predicatesExpressionList exp.ExpressionList

		if item.AllPredicatesMustMatch() {
			predicatesExpressionList = goqu.And(predicateExpressions...)
		} else {
			predicatesExpressionList = goqu.Or(predicateExpressions...)
		}

		itemsExpressions = append(
			itemsExpressions,
			goqu.And(eventTypesExpressionList, predicatesExpressionList),
		)
	}

	occurredAtExpressions := make([]goqu.Expression, 0)

	if !filter.OccurredFrom().IsZero() {
		occurredAtExpressions = append(
			occurredAtExpressions,
			goqu.C(colOccurredAt).Gte(filter.OccurredFrom()),
		)
	}

	if !filter.OccurredUntil().IsZero() {
		occurredAtExpressions = append(
			occurredAtExpressions,
			goqu.C(colOccurredAt).Lte(filter.OccurredUntil()),
		)
	}

	selectStmt = selectStmt.Where(
		goqu.And(
			goqu.Or(itemsExpressions...),
			goqu.And(occurredAtExpressions...),
		),
	)

	return selectStmt
}

func (es *EventStore) getBuilder() *goqu.DialectWrapper {
	return es.builderPool.Get().(*goqu.DialectWrapper)
}

func (es *EventStore) putBuilder(builder *goqu.DialectWrapper) {
	es.builderPool.Put(builder)
}

// isCancellationError checks if an error is due to context cancellation.
// Database drivers often wrap context errors improperly, so we use multiple detection strategies.
func (es *EventStore) isCancellationError(err error) bool {
	if err == nil {
		return false
	}

	// Fast path: check for properly wrapped errors
	if errors.Is(err, context.Canceled) {
		return true
	}

	// Fallback: check the error message for improperly wrapped errors
	// This handles cases where database drivers don't implement proper error wrapping
	errStr := err.Error()

	return strings.Contains(errStr, "context canceled") || strings.Contains(errStr, "context cancelled")
}

// isTimeoutError checks if an error is due to context deadline exceeded.
// Database drivers often wrap context errors improperly, so we use multiple detection strategies.
func (es *EventStore) isTimeoutError(err error) bool {
	if err == nil {
		return false
	}

	// Fast path: check for properly wrapped errors
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	// Fallback: check the error message for improperly wrapped errors
	// This handles cases where database drivers don't implement proper error wrapping
	errStr := err.Error()

	return strings.Contains(errStr, "context deadline exceeded")
}
