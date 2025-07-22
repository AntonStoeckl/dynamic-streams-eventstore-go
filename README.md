# Dynamic Streams EventStore

A **Go**-based _**Event Store**_ implementation for _**Event Sourcing**_ with PostgreSQL (17.5) as a storage engine,
which operates on the principal of **_Dynamic Event Streams_**.

What I simply call **_Dynamic Event Streams_** is currently discussed a lot as _**Dynamic Consistency Boundaries**_,
originally discussed and coined by [Sara Pellegrini](https://www.linkedin.com/in/sara-pellegrini-55a37913/).  
Check out https://sara.event-thinking.io/ for her ideas!

### Disclaimer

This is currently just an **experiment** with the concept, **not yet a production-ready library**
(well, it should actually work fine).  
It might be a production-ready OSS library at some point in the future, though.  
For the time being, use it or play with it if the idea is interesting for you. :-)

### Core ideas of Dynamic Event Streams

_Classical_ **_Event Stores_** have a _**fixed stream**_ with a _**StreamID**_ as a core concept.  
Each **Query** only retrieves events for one _**fixed stream**_.  
Equally, every **Append** only appends to one _**fixed stream**_ in one _**atomic operation**_ with _**optimistic locking**_,
that forms the _**consistency boundary**_.  
One _**atomic operation**_ with _**Optimistic locking**_ means that 1-N events are appended to the steam "all or nothing"
only when the stream has not changed since it was queried. To make this decision, the **_StreamVersion_** of the last query
must be sent with the **Append** operation.

### The problem which Dynamic Event Streams (or DCB) try to solve.

A _**fixed stream**_ is a tight boundary that also enforces designing with fixed **_Entities_** or **_Aggregates_**
(see: Domain-Driven Design).  
But there are often use-cases that require to modify multiple _**Event Streams**_ or **_Aggregates_** atomically.  
Just two examples, written as events:  
* _BookCopyLentToReader_ (a stream for each **BookCopy** and another one for each **Reader**)
* _StudentEnrolledToCourse_ (a stream for each **Student** and another one for each **Course**)

Using the first example, all we really need to implement a _**LendBookToReader**_ use-case are all (maybe just some) events
that affect the **BookCopy** or the **Reader**. We might want to be able to answer the following question (our **business logic**):
* Does the BookCopy exist?
* Is the BookCopy currently lent out?
* Is the Reader under their quota for lent book copies?

### Core ideas of this concrete implementation with Go and PostgreSQL

Standing on the shoulder of giants (as usual), I was inspired and got the core idea about how to implement **Append**  
in one atomic operation with optimistic locking by [Rico Fritsche](https://www.linkedin.com/in/ricofritzsche/).  
Specifically, from this article: https://ricofritzsche.me/how-i-built-an-aggregateless-event-store-with-typescript-and-postgresql/  
I share a lot of ideas with Rico, especially about _**Event Sourcing**_, _**Vertical Slices**_ and why OOP, CRUD, too much
abstraction, "clean" architecture(s), ... often create more problems than they solve.  
Rico is a much more productive writer than me, so make sure to check out his other [articles](https://ricofritzsche.me/)!

Long story short: the basics for this implementation are _**Guard Clauses**_ implemented with **_Common Table Expressions_** (CTE).  
The **Append** command uses the same where conditions that the **Query** used before, and with a CTE it guards that the
"dynamic stream" has not changed between **Query** and **Append**.

#### An example says more than 1000 words ...

The **Query** to read the dynamic event stream:

```postgresql
SELECT "event_type", "occurred_at", "payload", "metadata", "sequence_number"  
FROM "events"  
WHERE (
    (
           ("event_type" = 'BookCopyAddedToCirculation')
        OR ("event_type" = 'BookCopyLentToReader') 
        OR ("event_type" = 'BookCopyRemovedFromCirculation') 
        OR ("event_type" = 'BookCopyReturnedByReader')
    )
    AND
    (
           payload @> '{"BookID": "0198226e-19f6-7d29-8be9-10871e23e820"}'
        OR payload @> '{"ReaderID": "0198226e-19f6-7d2a-8342-dc5c8d5a2cd5"}'
    )
)  
ORDER BY "sequence_number" ASC 
```

The **Append** with the **CTE**:

```postgresql
WITH context AS --- CTE starts here
(
    SELECT MAX("sequence_number") AS "max_seq"
    FROM "events"  
    WHERE (
        (
               ("event_type" = 'BookCopyAddedToCirculation')
            OR ("event_type" = 'BookCopyLentToReader')
            OR ("event_type" = 'BookCopyRemovedFromCirculation')
            OR ("event_type" = 'BookCopyReturnedByReader')
        )
        AND
        (
               payload @> '{"BookID": "0198226e-19f6-7d29-8be9-10871e23e820"}'
            OR payload @> '{"ReaderID": "0198226e-19f6-7d2a-8342-dc5c8d5a2cd5"}'
        )
    )
) --- CTE ends here

INSERT INTO "events" ("event_type", "occurred_at", "payload", "metadata")  
SELECT 'BookCopyAddedToCirculation',
       '2025-07-22T10:28:50.382428Z',
       '{
         "BookID":"0198223b-12f8-74af-ada1-12ac0687b922",
         "ISBN":"978-1-098-10013-1",
         "Title":"Learning Domain-Driven Design",
         "Authors":"Vlad Khononov",
         "Edition":"First Edition",
         "Publisher":"O''Reilly Media, Inc.",
         "PublicationYear":2021,
         "OccurredAt":"2025-07-22T10:28:50.382428Z"
       }',
      '{"MessageID": "019831b5-a89b-7acd-84dc-866b984d2548", "CausationID": "019831b5-a89b-7acf-9fca-55ce950a4e5d", "CorrelationID": "019831b5-a89b-7ad0-8e89-9144002139c7"}'
FROM context WHERE (COALESCE("max_seq", 0) = 6)
```



SELECT "event_type", "payload", "metadata", "occurred_at", "sequence_number" FROM "events" WHERE ((("event_type" = 'BookCopyAddedToCirculation') OR ("event_type" = 'BookCopyLentToReader') OR ("event_type" = 'BookCopyRemovedFromCirculation') OR ("event_type" = 'BookCopyReturnedByReader')) AND payload @> '{"BookID": "019831ad-cc4c-7229-b520-235c023633b6"}') ORDER BY "sequence_number" ASC
WITH context AS (SELECT MAX("sequence_number") AS "max_seq" FROM "events" WHERE ((("event_type" = 'BookCopyAddedToCirculation') OR ("event_type" = 'BookCopyLentToReader') OR ("event_type" = 'BookCopyRemovedFromCirculation') OR ("event_type" = 'BookCopyReturnedByReader')) AND payload @> '{"BookID": "019831ad-cc4c-7229-b520-235c023633b6"}')) 
INSERT INTO "events" ("event_type", "occurred_at", "payload", "metadata") SELECT 'BookCopyRemovedFromCirculation', '2025-07-22T10:28:50.382428Z', '{"BookID":"019831ad-cc4c-7229-b520-235c023633b6","OccurredAt":"2025-07-22T10:28:50.382428Z"}', '{}' FROM "context" WHERE (COALESCE("max_seq", 0) = 2)





If you look close, you will notice that the where clause is the same in the **Query** and the **CTE**!  

At the point of the **Query** the highest sequence number of _**relevant events**_ was 6, so if it has changed
since then, no rows will be inserted (the guard). Which will then be mapped to a concurrency error in the event store.

So it's crucial that the same where clause is used in **Query** and **Append** (as mentioned above).  
I'll show an example under "Quick start for using it in an application" below.


## Features

### Production

- **PostgreSQL Backend engine**: Leverages PostgreSQL for reliable event storage
- **Event Filtering with a fluent FilterBuilder**: Storage-agnostic filtering capabilities for event queries
- **Mapping to StorableEvent**: A **StorableEvent** type with a factory method that is completely independent
  of the userland implementation of **DomainEvents** my just receiving the **EventType** and the serialized payload

#### Currently missing

- The table name (_events_) is currently hardcoded in the postgres engine implementation
- More storage engines, like MongoDB, might follow ...


## Tech Stack

- **Language**: Go 1.24
- **Database**: PostgreSQL (latest - 17.5 at the time of this writing)
- **Key Dependencies**:
    - `github.com/jackc/pgx/v5` - PostgreSQL driver
    - `github.com/doug-martin/goqu/v9` - SQL query builder
    - `github.com/google/uuid` - UUID generation
    - `github.com/json-iterator/go` - Fast JSON marshaling
    - `github.com/stretchr/testify/assert` - Assertions for testing


## Quick Start for running the tests

The project includes Docker Compose configuration with:
- **postgres_test**: Development database (port 5432)
- **postgres_benchmark**: Performance testing database (port 5433)

Both services include automatic database initialization from the `initdb/` directory.


1. **Start PostgreSQL for functional tests with Docker**: 
   ```bash
   docker-compose --file test/docker-compose.yml up -d postgres_test
   ```
   
2. **Start PostgreSQL for benchmark tests with Docker**: 
   ```bash
   docker-compose --file test/docker-compose.yml up -d postgres_benchmark
   ```
   
3. **Start both containers at once with Docker**:
   ```bash
   docker-compose --file test/docker-compose.yml
   ```

4. **Run Tests**: Execute the functional test suite
   ```bash
   go test ./eventstore/engine/
   ```

5. **Benchmarks**: Run performance benchmark test suite
   ```bash
   go test -bench=. ./eventstore/engine/
   ```


## Quick Start for using it in an application

Install the dependency in your Go application via go get (todo).

Check **test/initdb/init.sql** on how to set up tables and indexes.  
If you want to dockerize the event store DB, you can copy from **test/docker-compose.yml**.

### The fluent FilterBuilder

The FilterBuilder is designed with the idea to only allow "useful" filter combinations for event-sourced workflows;
this is clearly opinionated. It will guide the user by only allowing next operations that make sense.  
It is thoroughly documented, please see `eventstore/filter.go`.

Some examples (taken from `test/helper.go`) ...

This one might be useful for "feature slices" where only some events that are tied to an "entity" (BookCopy) are of interest:

```go
// WHERE ((eventType1 OR eventType2 OR ...) AND predicate)
filter := BuildEventFilter().
    Matching().
    AnyEventTypeOf(
        core.BookCopyAddedToCirculationEventType,
        core.BookCopyRemovedFromCirculationEventType,
        core.BookCopyLentToReaderEventType,
        core.BookCopyReturnedByReaderEventType).
    AndAnyPredicateOf(P("BookID", bookID.String())).
    Finalize()
}
```

This one "solves" the case(s) described above where two "entities" (BookCopy, Reader) are affected:

```go
// WHERE ((eventType1 OR eventType2 OR ...) AND (predicate1 OR predicate2))
filter := BuildEventFilter().
    Matching().
    AnyEventTypeOf(
        core.BookCopyAddedToCirculationEventType,
        core.BookCopyRemovedFromCirculationEventType,
        core.BookCopyLentToReaderEventType,
        core.BookCopyReturnedByReaderEventType).
    AndAnyPredicateOf(
        P("BookID", bookID.String()),
        P("ReaderID", readerID.String())).
    Finalize()
```

Another example that resembles classical/fixed streams (per aggregate/entity):

```go
filter := BuildEventFilter().
    Matching().
    AnyPredicateOf(P("BookID", bookID.String())).
    Finalize()
}
```

### Mapping from your DomainEvents to StorableEvent 

**_StorableEvent_** is completely agnostic of your implementation of DomainEvents, it just receives `(eventType string, payloadJSON []byte)`:

```go
payloadJSON, err := json.Marshal(event)
if err != nil {
	// handle error
}

esEvent := BuildStorableEvent(event.EventType(), payloadJSON)
```

### Putting it all together in an application

```go
// the code below should live in the "imperative shell"

filter := BuildEventFilter().
    Matching().
    AnyPredicateOf(P("BookID", bookID.String())).
    Finalize()
}

storableEvents, maxSequenceNumberBeforeAppend, queryErr := es.Query(filter)
if queryErr != nil {
    // handle error
}

domainEvents, mappingErr := shell.DomainEventsFrom(storableEvents)
if mappingErr != nil {
    // handle error
}

// See eventstore/engine/postgres_benchmark_test.go -> Benchmark_TypicalWorkload_With_Many_Events_InTheStore
//   for a business logic example. ApplyBusinessLogic() code should live in your "functional core".
event, bizErr := core.ApplyBusinessLogic(domainEvents)
if bizErr != nil {
    // handle error
}

payloadJSON, marshalingErr := json.Marshal(event)
if marshalingErr != nil {
	// handle error
}

esEvent := BuildStorableEvent(event.EventType(), payloadJSON)

appendErr := es.Append(esEvent, filter, maxSequenceNumberBeforeAppend)
if appendErr != nil {
    // handle error
}
```

### Userland code under test

The functional and benchmark tests use some stuff from test/userland that can be copied or used as inspiration.

#### test/userland/config

Contains Postgres DB config to be used with "github.com/jackc/pgx/v5/pgxpool" for testing and benchmarks.  
All values are hardcoded so that they correspond with test/docker-compose.yml.  
In a real application the values should be read from env or a .env file.

#### test/userland/core

**Core/shell** below follows the _**functional core, imperative shell**_ idea.  
Core contains an interface and some other bits for domain events in test/userland/core/domain_event.go.
The other files contain concrete domain event implementations.

#### test/userland/shell

Contains functions to unmarshal those domain events from **StorableEvent**(s).


## Benchmarks

When you run any benchmark for the first time, it will prime the DB with **one million events**, which runs
a while (**circa 1 hour** on my plain vanilla linux laptop).  
The docker image for benchmarks uses a persistent volume, so from then on this will not run, unless you 
delete the events (actually it checks if one million events exist) or delete the volume. A regular 
docker-compose down will keep the data intact.  
You can change the number of events to be set up, each benchmark has such a line:  
`factor := 1000 // multiplied by 1000 -> total num of fixture events`  
This is quite a naive implementation, but "good enough" for me at the moment.

### My benchmark results

I'm running this on an 8-core i7 with 16GB ram.  
The results naturally vary, I'm showing some "typical" results below.  
I'm running them with `--count 8` which means 8 repetitions.

```shell
goos: linux  
goarch: amd64  
pkg: dynamic-streams-eventstore/eventstore/engine  
cpu: Intel(R) Core(TM) i7-8565U CPU @ 1.80GHz  
Benchmark_Append_With_Many_Events_InTheStore/append-8 535 2211224 ns/op
Benchmark_Append_With_Many_Events_InTheStore/append-8 588 2494717 ns/op
Benchmark_Append_With_Many_Events_InTheStore/append-8 484 2560869 ns/op
Benchmark_Append_With_Many_Events_InTheStore/append-8 480 2566933 ns/op
Benchmark_Append_With_Many_Events_InTheStore/append-8 480 2477165 ns/op
Benchmark_Append_With_Many_Events_InTheStore/append-8 441 2556536 ns/op
Benchmark_Append_With_Many_Events_InTheStore/append-8 505 2605083 ns/op
Benchmark_Append_With_Many_Events_InTheStore/append-8 442 2623357 ns/op
```
```shell
goos: linux  
goarch: amd64  
pkg: dynamic-streams-eventstore/eventstore/engine  
cpu: Intel(R) Core(TM) i7-8565U CPU @ 1.80GHz  
Benchmark_Query_With_Many_Events_InTheStore/query-8 5818 197042 ns/op
Benchmark_Query_With_Many_Events_InTheStore/query-8 5202 204716 ns/op
Benchmark_Query_With_Many_Events_InTheStore/query-8 5222 192455 ns/op
Benchmark_Query_With_Many_Events_InTheStore/query-8 6264 190324 ns/op
Benchmark_Query_With_Many_Events_InTheStore/query-8 6236 192805 ns/op
Benchmark_Query_With_Many_Events_InTheStore/query-8 5370 209709 ns/op
Benchmark_Query_With_Many_Events_InTheStore/query-8 5589 204389 ns/op
Benchmark_Query_With_Many_Events_InTheStore/query-8 5571 193981 ns/op
Benchmark_Query_With_Many_Events_InTheStore/query-8 6292 194154 ns/op
Benchmark_Query_With_Many_Events_InTheStore/query-8 5716 210545 ns/op
```
```shell
goos: linux  
goarch: amd64  
pkg: dynamic-streams-eventstore/eventstore/engine  
cpu: Intel(R) Core(TM) i7-8565U CPU @ 1.80GHz  
Benchmark_TypicalWorkload_With_Many_Events_InTheStore  
Benchmark_TypicalWorkload_With_Many_Events_InTheStore/append  
Benchmark_TypicalWorkload_With_Many_Events_InTheStore/append
Benchmark_TypicalWorkload_With_Many_Events_InTheStore/append-8 302 3586976 ns/op
Benchmark_TypicalWorkload_With_Many_Events_InTheStore/append-8 339 3466188 ns/op
Benchmark_TypicalWorkload_With_Many_Events_InTheStore/append-8 331 3863671 ns/op
Benchmark_TypicalWorkload_With_Many_Events_InTheStore/append-8 290 4714235 ns/op
Benchmark_TypicalWorkload_With_Many_Events_InTheStore/append-8 226 4830584 ns/op
Benchmark_TypicalWorkload_With_Many_Events_InTheStore/append-8 267 5173156 ns/op
Benchmark_TypicalWorkload_With_Many_Events_InTheStore/append-8 217 5728742 ns/op
Benchmark_TypicalWorkload_With_Many_Events_InTheStore/append-8 204 5705906 ns/op
```

The "typical workload" benchmark does a full cycle of:
* Query events
* Unserialize events
* Apply business logic and make a decision
* Serialize event
* Append event

In other words, what a real application would do (minus http request, emitting events, ...).  
The average of those 8 "workloads" is around **4.6 ms**, which I consider decent on my hardware.


## License

This project is licensed under the terms specified in `LICENSE.txt` (**GNU GPLv3**).