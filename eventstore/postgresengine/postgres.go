package postgresengine

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/postgres"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jmoiron/sqlx"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine/internal/adapters"
)

const defaultEventTableName = "events"

type sqlQueryString = string

// Logger interface for SQL query logging, operational metrics, warnings, and error reporting
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

type EventStore struct {
	db             adapters.DBAdapter
	eventTableName string
	logger         Logger
}

// Option defines a functional option for configuring EventStore
type Option func(*EventStore) error

// WithTableName sets the table name for the EventStore
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
// Error level: Critical failures that cause operation failures
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

// NewEventStoreFromPGXPool creates a new EventStore using a pgx Pool with optional configuration
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

// NewEventStoreFromSQLDB creates a new EventStore using a sql.DB with optional configuration
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

// NewEventStoreFromSQLX creates a new EventStore using a sqlx.DB with optional configuration
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
			es.logger.Error("failed to build select query", "error", buildQueryErr.Error())
		}
		return empty, 0, buildQueryErr
	}

	start := time.Now()
	rows, queryErr := es.db.Query(ctx, sqlQuery)
	duration := time.Since(start)
	es.logQueryWithDuration(sqlQuery, "query", duration)

	if queryErr != nil {
		if es.logger != nil {
			es.logger.Error("database query execution failed", "error", queryErr.Error(), "query", sqlQuery)
		}
		return empty, 0, errors.Join(eventstore.ErrQueryingEventsFailed, queryErr)
	}

	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			if es.logger != nil {
				es.logger.Warn("failed to close database rows", "error", closeErr.Error())
			}
		}
	}()

	result := queryResultRow{}
	eventStream := make(eventstore.StorableEvents, 0)
	maxSequenceNumber := eventstore.MaxSequenceNumberUint(0)

	for rows.Next() {
		rowScanErr := rows.Scan(&result.eventType, &result.occurredAt, &result.payload, &result.metadata, &result.maxSequenceNumber)
		if rowScanErr != nil {
			if es.logger != nil {
				es.logger.Error("failed to scan database row", "error", rowScanErr.Error())
			}
			return empty, 0, errors.Join(eventstore.ErrScanningDBRowFailed, rowScanErr)
		}

		event, buildStorableErr := eventstore.BuildStorableEvent(result.eventType, result.occurredAt, result.payload, result.metadata)
		eventStream = append(
			eventStream,
			event,
		)

		if buildStorableErr != nil {
			if es.logger != nil {
				es.logger.Error("failed to build storable event from database row", "error", buildStorableErr.Error(), "event_type", result.eventType)
			}
			return empty, 0, errors.Join(eventstore.ErrBuildingStorableEventFailed, buildStorableErr)
		}

		maxSequenceNumber = result.maxSequenceNumber
	}

	es.logOperation(
		"query completed",
		"event_count", len(eventStream),
		"duration_ms", es.durationToMilliseconds(duration))

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

	var sqlQuery sqlQueryString
	var buildQueryErr error

	switch len(allEvents) {
	case 1:
		sqlQuery, buildQueryErr = es.buildInsertQueryForSingleEvent(event, filter, expectedMaxSequenceNumber)
	default:
		sqlQuery, buildQueryErr = es.buildInsertQueryForMultipleEvents(allEvents, filter, expectedMaxSequenceNumber)
	}

	if buildQueryErr != nil {
		if es.logger != nil {
			es.logger.Error("failed to build insert query", "error", buildQueryErr.Error(), "event_count", len(allEvents))
		}
		return buildQueryErr
	}

	start := time.Now()
	tag, execErr := es.db.Exec(ctx, sqlQuery)
	duration := time.Since(start)
	es.logQueryWithDuration(sqlQuery, "append", duration)

	if execErr != nil {
		if es.logger != nil {
			es.logger.Error("database execution failed during event append", "error", execErr.Error(), "query", sqlQuery)
		}
		return errors.Join(eventstore.ErrAppendingEventFailed, execErr)
	}

	rowsAffected, rowsAffectedErr := tag.RowsAffected()
	if rowsAffectedErr != nil {
		if es.logger != nil {
			es.logger.Error("failed to get rows affected count", "error", rowsAffectedErr.Error())
		}
		return errors.Join(eventstore.ErrGettingRowsAffectedFailed, rowsAffectedErr)
	}

	if rowsAffected < int64(len(allEvents)) {
		es.logOperation(
			"concurrency conflict detected",
			"expected_events", len(allEvents),
			"rows_affected", rowsAffected,
			"expected_sequence", expectedMaxSequenceNumber,
		)

		return eventstore.ErrConcurrencyConflict
	}

	es.logOperation(
		"events appended",
		"event_count", len(allEvents),
		"duration_ms", es.durationToMilliseconds(duration),
	)

	return nil
}

func (es EventStore) buildSelectQuery(filter eventstore.Filter) (sqlQueryString, error) {
	selectStmt := goqu.Dialect("postgres").
		From(es.eventTableName).
		Select("event_type", "occurred_at", "payload", "metadata", "sequence_number").
		Order(goqu.I("sequence_number").Asc())

	selectStmt = es.addWhereClause(filter, selectStmt)

	sqlQuery, _, toSqlErr := selectStmt.ToSQL()
	if toSqlErr != nil {
		return "", errors.Join(eventstore.ErrBuildingQueryFailed, toSqlErr)
	}

	return sqlQuery, nil
}

func (es EventStore) buildInsertQueryForSingleEvent(
	event eventstore.StorableEvent,
	filter eventstore.Filter,
	expectedMaxSequenceNumber eventstore.MaxSequenceNumberUint,
) (sqlQueryString, error) {

	builder := goqu.Dialect("postgres")

	// Define the subquery for the CTE
	cteStmt := builder.
		From(es.eventTableName).
		Select(goqu.MAX("sequence_number").As("max_seq"))

	cteStmt = es.addWhereClause(filter, cteStmt)

	// Define the SELECT for the INSERT
	selectStmt := builder.
		From("context").
		Select(goqu.V(event.EventType), goqu.V(event.OccurredAt), goqu.V(event.PayloadJSON), goqu.V(event.MetadataJSON)).
		Where(goqu.COALESCE(goqu.C("max_seq"), 0).Eq(goqu.V(expectedMaxSequenceNumber)))

	// Finalize the full INSERT query
	insertStmt := builder.
		Insert(es.eventTableName).
		Cols("event_type", "occurred_at", "payload", "metadata").
		FromQuery(selectStmt).
		With("context", cteStmt)

	sqlQuery, _, toSqlErr := insertStmt.ToSQL()
	if toSqlErr != nil {
		if es.logger != nil {
			es.logger.Error("failed to convert single event insert statement to SQL", "error", toSqlErr.Error(), "event_type", event.EventType)
		}
		return "", errors.Join(eventstore.ErrBuildingQueryFailed, toSqlErr)
	}

	return sqlQuery, nil
}

func (es EventStore) buildInsertQueryForMultipleEvents(
	events []eventstore.StorableEvent,
	filter eventstore.Filter,
	expectedMaxSequenceNumber eventstore.MaxSequenceNumberUint,
) (sqlQueryString, error) {
	builder := goqu.Dialect("postgres")

	// Define the subquery for the CTE
	cteStmt := builder.
		From(es.eventTableName).
		Select(goqu.MAX("sequence_number").As("max_seq"))

	cteStmt = es.addWhereClause(filter, cteStmt)

	// Create individual SELECT statements for each event
	unionStatements := make([]*goqu.SelectDataset, len(events))
	for i, event := range events {
		unionStatements[i] = builder.
			Select(
				goqu.L("?::text", event.EventType).As("event_type"),
				goqu.L("?::timestamp with time zone", event.OccurredAt).As("occurred_at"),
				goqu.L("?::jsonb", event.PayloadJSON).As("payload"),
				goqu.L("?::jsonb", event.MetadataJSON).As("metadata"),
			)
	}

	// Combine all SELECT statements with UNION ALL
	valuesStmt := unionStatements[0]
	for i := 1; i < len(unionStatements); i++ {
		valuesStmt = valuesStmt.UnionAll(unionStatements[i])
	}

	// Finalize the full INSERT query
	insertStmt := builder.
		Insert(es.eventTableName).
		Cols("event_type", "occurred_at", "payload", "metadata").
		With("context", cteStmt).
		With("vals", valuesStmt).
		FromQuery(
			builder.From("context", "vals").
				Select("vals.event_type", "vals.occurred_at", "vals.payload", "vals.metadata").
				Where(goqu.COALESCE(goqu.C("max_seq"), 0).Eq(goqu.V(expectedMaxSequenceNumber))),
		)

	sqlQuery, _, toSqlErr := insertStmt.ToSQL()
	if toSqlErr != nil {
		if es.logger != nil {
			es.logger.Error("failed to convert multiple events insert statement to SQL", "error", toSqlErr.Error(), "event_count", len(events))
		}
		return "", errors.Join(eventstore.ErrBuildingQueryFailed, toSqlErr)
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
				goqu.Ex{"event_type": eventType},
			)
		}

		// eventTypes must always be filtered with OR ;-)
		eventTypesExpressionList := goqu.Or(eventTypeExpressions...)

		for _, predicate := range item.Predicates() {
			predicateExpressions = append(
				predicateExpressions,
				goqu.L(fmt.Sprintf(`payload @> '{"%s": "%s"}'`, predicate.Key(), predicate.Val())),
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
			goqu.C("occurred_at").Gte(filter.OccurredFrom()),
		)
	}

	if !filter.OccurredUntil().IsZero() {
		occurredAtExpressions = append(
			occurredAtExpressions,
			goqu.C("occurred_at").Lte(filter.OccurredUntil()),
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

// logQueryWithDuration logs SQL queries with execution time at debug level if the logger is configured
func (es EventStore) logQueryWithDuration(sqlQuery string, action string, duration time.Duration) {
	if es.logger != nil {
		es.logger.Debug("executed sql for: "+action, "duration_ms", es.durationToMilliseconds(duration), "query", sqlQuery)
	}
}

// logOperation logs operational information at info level if the logger is configured
func (es EventStore) logOperation(action string, args ...any) {
	if es.logger != nil {
		es.logger.Info("eventstore operation: "+action, args...)
	}
}

// durationToMilliseconds converts a time.Duration to float64 milliseconds with 3 decimal places
func (es EventStore) durationToMilliseconds(d time.Duration) float64 {
	return math.Round(float64(d.Nanoseconds())/1e6*1000) / 1000
}
