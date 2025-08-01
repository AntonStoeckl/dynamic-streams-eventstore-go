package postgresengine

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
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
	defaultEventTableName          = "events"
	logMsgBuildSelectQueryFailed   = "failed to build select query"
	logMsgDBQueryFailed            = "database query execution failed"
	logMsgCloseRowsFailed          = "failed to close database rows"
	logMsgScanRowFailed            = "failed to scan database row"
	logMsgBuildStorableEventFailed = "failed to build storable event from database row"
	logMsgBuildInsertQueryFailed   = "failed to build insert query"
	logMsgDBExecFailed             = "database execution failed during event append"
	logMsgRowsAffectedFailed       = "failed to get rows affected count"
	logMsgSingleEventSQLFailed     = "failed to convert single event insert statement to SQL"
	logMsgMultiEventSQLFailed      = "failed to convert multiple events insert statement to SQL"
	logMsgQueryCompleted           = "query completed"
	logMsgEventsAppended           = "events appended"
	logMsgConcurrencyConflict      = "concurrency conflict detected"
	logMsgSQLExecuted              = "executed sql for: "
	logMsgOperation                = "eventstore operation: "
	logAttrError                   = "error"
	logAttrQuery                   = "query"
	logAttrEventType               = "event_type"
	logAttrEventCount              = "event_count"
	logAttrDurationMS              = "duration_ms"
	logAttrExpectedEvents          = "expected_events"
	logAttrRowsAffected            = "rows_affected"
	logAttrExpectedSequence        = "expected_sequence"
	logActionQuery                 = "query"
	logActionAppend                = "append"
	colEventType                   = "event_type"
	colOccurredAt                  = "occurred_at"
	colPayload                     = "payload"
	colMetadata                    = "metadata"
	colSequenceNumber              = "sequence_number"
	cteContext                     = "context"
	cteVals                        = "vals"
	dialectPostgres                = "postgres"
	aliasMaxSeq                    = "max_seq"
	castText                       = "?::text"
	castTimestamp                  = "?::timestamp with time zone"
	castJsonb                      = "?::jsonb"
)

type (
	sqlQueryString    = string
	rowsAffectedInt64 = int64
	queryDuration     = time.Duration
)

// Logger interface for SQL query logging, operational metrics, warnings, and error reporting.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// EventStore represents a storage mechanism for handling and querying events in an event sourcing implementation.
// It leverages a database adapter and supports customizable logging and event table configuration.
type EventStore struct {
	db             adapters.DBAdapter
	eventTableName string
	logger         Logger
}

// Option defines a functional option for configuring EventStore.
type Option func(*EventStore) error

// WithTableName sets the table name for the EventStore.
func WithTableName(tableName string) Option {
	return func(es *EventStore) error {
		if tableName == "" {
			return eventstore.ErrEmptyEventsTableName
		}

		es.eventTableName = tableName

		return nil
	}
}

// WithLogger sets the logger for the EventStore.
// The logger will receive messages at different levels based on the logger's configured level:
//
// Debug level: SQL queries with execution timing (development use)
// Info level: Event counts, durations, concurrency conflicts (production-safe)
// Warn level: Non-critical issues like cleanup failures
// Error level: Critical failures that cause operation failures.
func WithLogger(logger Logger) Option {
	return func(es *EventStore) error {
		es.logger = logger
		return nil
	}
}

type queryResultRow struct {
	eventType         string
	payload           []byte
	metadata          []byte
	occurredAt        time.Time
	maxSequenceNumber eventstore.MaxSequenceNumberUint
}

// NewEventStoreFromPGXPool creates a new EventStore using a pgx Pool with optional configuration.
func NewEventStoreFromPGXPool(db *pgxpool.Pool, options ...Option) (EventStore, error) {
	if db == nil {
		return EventStore{}, eventstore.ErrNilDatabaseConnection
	}

	es := EventStore{
		db:             adapters.NewPGXAdapter(db),
		eventTableName: defaultEventTableName,
	}

	for _, option := range options {
		if err := option(&es); err != nil {
			return EventStore{}, err
		}
	}

	return es, nil
}

// NewEventStoreFromSQLDB creates a new EventStore using a sql.DB with optional configuration.
func NewEventStoreFromSQLDB(db *sql.DB, options ...Option) (EventStore, error) {
	if db == nil {
		return EventStore{}, eventstore.ErrNilDatabaseConnection
	}

	es := EventStore{
		db:             adapters.NewSQLAdapter(db),
		eventTableName: defaultEventTableName,
	}

	for _, option := range options {
		if err := option(&es); err != nil {
			return EventStore{}, err
		}
	}

	return es, nil
}

// NewEventStoreFromSQLX creates a new EventStore using a sqlx.DB with optional configuration.
func NewEventStoreFromSQLX(db *sqlx.DB, options ...Option) (EventStore, error) {
	if db == nil {
		return EventStore{}, eventstore.ErrNilDatabaseConnection
	}

	es := EventStore{
		db:             adapters.NewSQLXAdapter(db),
		eventTableName: defaultEventTableName,
	}

	for _, option := range options {
		if err := option(&es); err != nil {
			return EventStore{}, err
		}
	}

	return es, nil
}

// Query retrieves events from the Postgres event store based on the provided eventstore.Filter criteria
// and returns them as eventstore.StorableEvents
// as well as the MaxSequenceNumberUint for this "dynamic event stream" at the time of the query.
func (es EventStore) Query(ctx context.Context, filter eventstore.Filter) (
	eventstore.StorableEvents,
	eventstore.MaxSequenceNumberUint,
	error,
) {

	var empty eventstore.StorableEvents

	sqlQuery, buildQueryErr := es.buildSelectQuery(filter)
	if buildQueryErr != nil {
		if es.logger != nil {
			es.logger.Error(logMsgBuildSelectQueryFailed, logAttrError, buildQueryErr.Error())
		}
		return empty, 0, buildQueryErr
	}

	rows, duration, queryErr := es.executeQuery(ctx, sqlQuery)
	if queryErr != nil {
		return empty, 0, queryErr
	}
	defer es.closeRows(rows)

	eventStream, maxSequenceNumber, scanErr := es.processQueryResults(rows)
	if scanErr != nil {
		return empty, 0, scanErr
	}

	es.logOperation(
		logMsgQueryCompleted,
		logAttrEventCount, len(eventStream),
		logAttrDurationMS, es.durationToMilliseconds(duration))

	return eventStream, maxSequenceNumber, nil
}

// executeQuery executes the SQL query and returns rows with timing information.
func (es EventStore) executeQuery(ctx context.Context, sqlQuery string) (
	adapters.DBRows,
	time.Duration,
	error,
) {

	start := time.Now()
	rows, queryErr := es.db.Query(ctx, sqlQuery)
	duration := time.Since(start)
	es.logQueryWithDuration(sqlQuery, logActionQuery, duration)

	if queryErr != nil {
		if es.logger != nil {
			es.logger.Error(logMsgDBQueryFailed, logAttrError, queryErr.Error(), logAttrQuery, sqlQuery)
		}

		return nil, duration, errors.Join(eventstore.ErrQueryingEventsFailed, queryErr)
	}

	return rows, duration, nil
}

// closeRows safely closes database rows and logs any errors.
func (es EventStore) closeRows(rows adapters.DBRows) {
	if closeErr := rows.Close(); closeErr != nil {
		if es.logger != nil {
			es.logger.Warn(logMsgCloseRowsFailed, logAttrError, closeErr.Error())
		}
	}
}

// processQueryResults processes database rows and converts them to domain events.
func (es EventStore) processQueryResults(rows adapters.DBRows) (
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
			if es.logger != nil {
				es.logger.Error(logMsgScanRowFailed, logAttrError, rowScanErr.Error())
			}

			return empty, 0, errors.Join(eventstore.ErrScanningDBRowFailed, rowScanErr)
		}

		event, buildStorableErr := eventstore.BuildStorableEvent(result.eventType, result.occurredAt, result.payload, result.metadata)
		if buildStorableErr != nil {
			if es.logger != nil {
				es.logger.Error(logMsgBuildStorableEventFailed, logAttrError, buildStorableErr.Error(), logAttrEventType, result.eventType)
			}

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
func (es EventStore) Append(
	ctx context.Context,
	filter eventstore.Filter,
	expectedMaxSequenceNumber eventstore.MaxSequenceNumberUint,
	event eventstore.StorableEvent,
	additionalEvents ...eventstore.StorableEvent,
) error {

	allEvents := eventstore.StorableEvents{event}
	allEvents = append(allEvents, additionalEvents...)

	sqlQuery, buildQueryErr := es.buildAppendQuery(allEvents, filter, expectedMaxSequenceNumber)
	if buildQueryErr != nil {
		return buildQueryErr
	}

	rowsAffected, duration, execErr := es.executeAppendQuery(ctx, sqlQuery)
	if execErr != nil {
		return execErr
	}

	if err := es.validateAppendResult(rowsAffected, len(allEvents), expectedMaxSequenceNumber); err != nil {
		return err
	}

	es.logOperation(
		logMsgEventsAppended,
		logAttrEventCount, len(allEvents),
		logAttrDurationMS, es.durationToMilliseconds(duration),
	)

	return nil
}

// buildAppendQuery builds the appropriate SQL query for single or multiple events.
func (es EventStore) buildAppendQuery(
	allEvents eventstore.StorableEvents,
	filter eventstore.Filter,
	expectedMaxSequenceNumber eventstore.MaxSequenceNumberUint,
) (sqlQueryString, error) {

	var sqlQuery sqlQueryString
	var buildQueryErr error

	switch len(allEvents) {
	case 1:
		sqlQuery, buildQueryErr = es.buildInsertQueryForSingleEvent(allEvents[0], filter, expectedMaxSequenceNumber)

	default:
		sqlQuery, buildQueryErr = es.buildInsertQueryForMultipleEvents(allEvents, filter, expectedMaxSequenceNumber)
	}

	if buildQueryErr != nil {
		if es.logger != nil {
			es.logger.Error(logMsgBuildInsertQueryFailed, logAttrError, buildQueryErr.Error(), logAttrEventCount, len(allEvents))
		}

		return "", buildQueryErr
	}

	return sqlQuery, nil
}

// executeAppendQuery executes the SQL append query and returns rows affected and duration.
func (es EventStore) executeAppendQuery(ctx context.Context, sqlQuery string) (
	rowsAffectedInt64,
	queryDuration,
	error,
) {

	start := time.Now()
	tag, execErr := es.db.Exec(ctx, sqlQuery)
	duration := time.Since(start)
	es.logQueryWithDuration(sqlQuery, logActionAppend, duration)

	if execErr != nil {
		if es.logger != nil {
			es.logger.Error(logMsgDBExecFailed, logAttrError, execErr.Error(), logAttrQuery, sqlQuery)
		}

		return 0, duration, errors.Join(eventstore.ErrAppendingEventFailed, execErr)
	}

	rowsAffected, rowsAffectedErr := tag.RowsAffected()
	if rowsAffectedErr != nil {
		if es.logger != nil {
			es.logger.Error(logMsgRowsAffectedFailed, logAttrError, rowsAffectedErr.Error())
		}

		return 0, duration, errors.Join(eventstore.ErrGettingRowsAffectedFailed, rowsAffectedErr)
	}

	return rowsAffected, duration, nil
}

// validateAppendResult checks if the append operation was successful and detects concurrency conflicts.
func (es EventStore) validateAppendResult(
	rowsAffected int64,
	expectedEventCount int,
	expectedMaxSequenceNumber eventstore.MaxSequenceNumberUint,
) error {

	if rowsAffected < int64(expectedEventCount) {
		es.logOperation(
			logMsgConcurrencyConflict,
			logAttrExpectedEvents, expectedEventCount,
			logAttrRowsAffected, rowsAffected,
			logAttrExpectedSequence, expectedMaxSequenceNumber,
		)

		return eventstore.ErrConcurrencyConflict
	}

	return nil
}

func (es EventStore) buildSelectQuery(filter eventstore.Filter) (sqlQueryString, error) {
	selectStmt := goqu.Dialect(dialectPostgres).
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

func (es EventStore) buildInsertQueryForSingleEvent(
	event eventstore.StorableEvent,
	filter eventstore.Filter,
	expectedMaxSequenceNumber eventstore.MaxSequenceNumberUint,
) (sqlQueryString, error) {

	builder := goqu.Dialect(dialectPostgres)

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
		if es.logger != nil {
			es.logger.Error(logMsgSingleEventSQLFailed, logAttrError, toSQLErr.Error(), logAttrEventType, event.EventType)
		}
		return "", errors.Join(eventstore.ErrBuildingQueryFailed, toSQLErr)
	}

	return sqlQuery, nil
}

func (es EventStore) buildInsertQueryForMultipleEvents(
	events []eventstore.StorableEvent,
	filter eventstore.Filter,
	expectedMaxSequenceNumber eventstore.MaxSequenceNumberUint,
) (sqlQueryString, error) {

	builder := goqu.Dialect(dialectPostgres)

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
		if es.logger != nil {
			es.logger.Error(logMsgMultiEventSQLFailed, logAttrError, toSQLErr.Error(), logAttrEventCount, len(events))
		}
		return "", errors.Join(eventstore.ErrBuildingQueryFailed, toSQLErr)
	}

	return sqlQuery, nil
}

func (es EventStore) addWhereClause(filter eventstore.Filter, selectStmt *goqu.SelectDataset) *goqu.SelectDataset {
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

// logQueryWithDuration logs SQL queries with execution time at debug level if the logger is configured.
func (es EventStore) logQueryWithDuration(
	sqlQuery string,
	action string,
	duration time.Duration,
) {

	if es.logger != nil {
		es.logger.Debug(logMsgSQLExecuted+action, logAttrDurationMS, es.durationToMilliseconds(duration), logAttrQuery, sqlQuery)
	}
}

// logOperation logs operational information at info level if the logger is configured.
func (es EventStore) logOperation(action string, args ...any) {
	if es.logger != nil {
		es.logger.Info(logMsgOperation+action, args...)
	}
}

// durationToMilliseconds converts a time.Duration to float64 milliseconds with 3 decimal places.
func (es EventStore) durationToMilliseconds(d time.Duration) float64 {
	return math.Round(float64(d.Nanoseconds())/1e6*1000) / 1000
}
