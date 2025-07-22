package engine

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/postgres"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/jackc/pgx/v5/pgxpool"

	. "dynamic-streams-eventstore/eventstore"
)

var ErrConcurrencyConflict = errors.New("concurrency error, no rows were affected")

// MaxSequenceNumberUint is a type alias for uint, representing the maximum sequence number for a "dynamic event stream".
type MaxSequenceNumberUint = uint

type sqlQueryString = string

type PostgresEventStore struct {
	db *pgxpool.Pool
}

type queryResultRow struct {
	eventType         string
	payload           []byte
	metadata          []byte
	occurredAt        time.Time
	maxSequenceNumber MaxSequenceNumberUint
}

func NewPostgresEventStore(db *pgxpool.Pool) PostgresEventStore {
	return PostgresEventStore{db: db}
}

// Query retrieves events from the Postgres event store based on the provided eventstore.Filter criteria
// and returns them as eventstore.StorableEvents
// as well as the MaxSequenceNumberUint for this "dynamic event stream" at the time of the query.
func (es PostgresEventStore) Query(filter Filter) (StorableEvents, MaxSequenceNumberUint, error) {
	empty := make(StorableEvents, 0)

	sqlQuery, buildQueryErr := es.buildSelectQuery(filter)
	if buildQueryErr != nil {
		return empty, 0, buildQueryErr
	}

	//fmt.Println(sqlQuery)

	rows, queryErr := es.db.Query(context.Background(), sqlQuery)
	if queryErr != nil {
		return empty, 0, errors.Join(errors.New("querying events failed"), queryErr)
	}

	defer rows.Close()

	result := queryResultRow{}
	eventStream := make(StorableEvents, 0)
	maxSequenceNumber := MaxSequenceNumberUint(0)

	for rows.Next() {
		rowScanErr := rows.Scan(&result.eventType, &result.occurredAt, &result.payload, &result.metadata, &result.maxSequenceNumber)
		if rowScanErr != nil {
			return empty, 0, errors.Join(errors.New("scanning db row failed"), rowScanErr)
		}

		eventStream = append(
			eventStream,
			BuildStorableEvent(result.eventType, result.occurredAt, result.payload, result.metadata),
		)

		maxSequenceNumber = result.maxSequenceNumber
	}

	return eventStream, maxSequenceNumber, nil
}

// Append attempts to append an eventstore.StorableEvent onto the Postgres event store respecting concurrency constraints
// for this "dynamic event stream" based on the provided eventstore.Filter criteria and the expected MaxSequenceNumberUint.
//
// The provided eventstore.Filter criteria should be the same as the ones used for the Query before making the business decisions.
func (es PostgresEventStore) Append(
	event StorableEvent,
	filter Filter,
	expectedMaxSequenceNumber MaxSequenceNumberUint,
) error {

	sqlQuery, buildQueryErr := es.buildInsertQuery(event, filter, expectedMaxSequenceNumber)
	if buildQueryErr != nil {
		return buildQueryErr
	}

	//fmt.Println(sqlQuery)

	tag, execErr := es.db.Exec(context.Background(), sqlQuery)
	if execErr != nil {
		return errors.Join(errors.New("appending the event failed"), execErr)
	}

	if tag.RowsAffected() < 1 {
		return ErrConcurrencyConflict
	}

	return nil
}

func (es PostgresEventStore) buildSelectQuery(filter Filter) (sqlQueryString, error) {
	selectStmt := goqu.Dialect("postgres").
		From("events").
		Select("event_type", "occurred_at", "payload", "metadata", "sequence_number").
		Order(goqu.I("sequence_number").Asc())

	selectStmt = es.addWhereClause(filter, selectStmt)

	sqlQuery, _, toSqlErr := selectStmt.ToSQL()
	if toSqlErr != nil {
		return "", errors.Join(errors.New("building the query failed"), toSqlErr)
	}

	return sqlQuery, nil
}

func (es PostgresEventStore) buildInsertQuery(
	event StorableEvent,
	filter Filter,
	expectedMaxSequenceNumber MaxSequenceNumberUint,
) (sqlQueryString, error) {

	builder := goqu.Dialect("postgres")

	// Define the subquery for the CTE
	cteStmt := builder.
		From("events").
		Select(goqu.MAX("sequence_number").As("max_seq"))

	cteStmt = es.addWhereClause(filter, cteStmt)

	// Define the SELECT for the INSERT
	selectStmt := builder.
		From("context").
		Select(goqu.V(event.EventType), goqu.V(event.OccurredAt), goqu.V(event.PayloadJSON), goqu.V(event.MetadataJSON)).
		Where(goqu.COALESCE(goqu.C("max_seq"), 0).Eq(goqu.V(expectedMaxSequenceNumber)))

	// Finalize the full INSERT query
	insertStmt := builder.
		Insert("events").
		Cols("event_type", "occurred_at", "payload", "metadata").
		FromQuery(selectStmt).
		With("context", cteStmt)

	sqlQuery, _, toSqlErr := insertStmt.ToSQL()
	if toSqlErr != nil {
		return "", errors.Join(errors.New("building the query failed"), toSqlErr)
	}

	return sqlQuery, nil
}

func (es PostgresEventStore) addWhereClause(filter Filter, selectStmt *goqu.SelectDataset) *goqu.SelectDataset {
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
