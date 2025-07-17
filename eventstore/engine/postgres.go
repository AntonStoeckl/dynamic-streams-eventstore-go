package engine

import (
	"context"
	"errors"
	"fmt"

	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/postgres"
	"github.com/jackc/pgx/v5/pgxpool"

	"dynamic-streams-eventstore/eventstore"
	"dynamic-streams-eventstore/test/userland/core"
	"dynamic-streams-eventstore/test/userland/shell"
)

var ErrConcurrencyConflict = errors.New("no rows were affected")

type unmarshalEventFromJSON func(eventType string, payload []byte) (core.Event, error)

type PostgresEventStore struct {
	db                     *pgxpool.Pool
	unmarshalEventFromJSON unmarshalEventFromJSON
}

type eventStreamSlice = []core.Event
type maxSequenceNumberUint = uint
type sqlQueryString = string

type queryResultRow struct {
	eventType         string
	payload           []byte
	maxSequenceNumber uint
}

func NewPostgresEventStore(db *pgxpool.Pool, unmarshalEventFromJSON unmarshalEventFromJSON) PostgresEventStore {
	return PostgresEventStore{
		db:                     db,
		unmarshalEventFromJSON: unmarshalEventFromJSON,
	}
}

func (es PostgresEventStore) Query(filter eventstore.Filter) ([]core.Event, uint, error) {
	sqlQuery, buildQueryErr := es.buildSelectQuery(filter)
	if buildQueryErr != nil {
		return nil, 0, buildQueryErr
	}

	//fmt.Println(sqlQuery)

	rows, queryErr := es.db.Query(context.Background(), sqlQuery)
	if queryErr != nil {
		return nil, 0, errors.Join(errors.New("querying events failed"), queryErr)
	}

	defer rows.Close()

	result := queryResultRow{}
	eventStream := make(eventStreamSlice, 0)
	maxSequenceNumber := maxSequenceNumberUint(0)

	for rows.Next() {
		rowScanErr := rows.Scan(&result.eventType, &result.payload, &result.maxSequenceNumber)
		if rowScanErr != nil {
			return eventStreamSlice{}, 0, errors.Join(errors.New("scanning db row failed"), rowScanErr)
		}

		event, mapToEventErr := shell.EventFromJSON(result.eventType, result.payload)
		if mapToEventErr != nil {
			return eventStreamSlice{}, maxSequenceNumberUint(0), mapToEventErr
		}

		eventStream = append(eventStream, event)
		maxSequenceNumber = result.maxSequenceNumber
	}

	return eventStream, maxSequenceNumber, nil
}

func (es PostgresEventStore) Append(
	event core.Event,
	filter eventstore.Filter,
	expectedMaxSequenceNumber maxSequenceNumberUint,
) error {

	payloadJSON, unmarshalErr := event.PayloadToJSON()
	if unmarshalErr != nil {
		return errors.Join(errors.New("marshaling the event payload failed"), unmarshalErr)
	}

	sqlQuery, buildQueryErr := es.buildInsertQuery(event, filter, expectedMaxSequenceNumber, payloadJSON)
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

func (es PostgresEventStore) buildSelectQuery(filter eventstore.Filter) (sqlQueryString, error) {
	selectStmt := goqu.Dialect("postgres").
		From("events").
		Select("event_type", "payload", "sequence_number").
		Order(goqu.I("sequence_number").Asc())

	selectStmt = es.addWhereClause(filter, selectStmt)

	sqlQuery, _, toSqlErr := selectStmt.ToSQL()
	if toSqlErr != nil {
		return "", errors.Join(errors.New("building the query failed"), toSqlErr)
	}

	return sqlQuery, nil
}

func (es PostgresEventStore) buildInsertQuery(
	event core.Event,
	filter eventstore.Filter,
	expectedMaxSequenceNumber maxSequenceNumberUint,
	payloadJSON []byte,
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
		Select(goqu.V(event.EventType()), goqu.V(payloadJSON)).
		Where(goqu.COALESCE(goqu.C("max_seq"), 0).Eq(goqu.V(expectedMaxSequenceNumber)))

	// Finalize the full INSERT query
	insertStmt := builder.
		Insert("events").
		Cols("event_type", "payload").
		FromQuery(selectStmt).
		With("context", cteStmt)

	sqlQuery, _, toSqlErr := insertStmt.ToSQL()
	if toSqlErr != nil {
		return "", errors.Join(errors.New("building the query failed"), toSqlErr)
	}

	return sqlQuery, nil
}

func (es PostgresEventStore) addWhereClause(filter eventstore.Filter, selectStmt *goqu.SelectDataset) *goqu.SelectDataset {
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

		for _, predicate := range item.Predicates() {
			predicateExpressions = append(
				predicateExpressions,
				goqu.L(fmt.Sprintf(`payload @> '{"%s": "%s"}'`, predicate.Key(), predicate.Val())),
			)
		}

		itemsExpressions = append(
			itemsExpressions,
			goqu.And(
				goqu.Or(eventTypeExpressions...),
				goqu.Or(predicateExpressions...),
			))
	}

	selectStmt = selectStmt.Where(goqu.Or(itemsExpressions...))

	return selectStmt
}
