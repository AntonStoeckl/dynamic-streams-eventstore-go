# Dynamic Streams EventStore

A **Go**-based _**Event Store**_ implementation for _**Event Sourcing**_ with PostgreSQL (17.5) as a storage engine,
which operates on the principal of **_Dynamic Event Streams_**.

What I simply call **_Dynamic Event Streams_** is currently discussed a lot as _**Dynamic Consistency Boundaries**_,
originally discussed and coined by [Sara Pellegrini](https://www.linkedin.com/in/sara-pellegrini-55a37913/).  
Check out https://sara.event-thinking.io/ for her ideas!

### Disclaimer

This is currently just an **experiment** with the concept, **not yet a production-ready library**.  
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

### The problem which Dynamic Event Streams or DCB try to solve.

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
"dynamic stream" has not changed between Query and Append.

An example speaks more than 1000 words ...

The **Query** to read the event stream:

`SELECT "event_type", "payload", "sequence_number"`  
`FROM "events"`  
`WHERE ((("event_type" = 'BookCopyAddedToCirculation') OR ("event_type" = 'BookCopyLentToReader') OR ("event_type" = 'BookCopyRemovedFromCirculation') OR ("event_type" = 'BookCopyReturnedByReader')) AND (payload @> '{"BookID": "0198226e-19f6-7d29-8be9-10871e23e820"}' OR payload @> '{"ReaderID": "0198226e-19f6-7d2a-8342-dc5c8d5a2cd5"}'))`  
`ORDER BY "sequence_number" ASC `

The **CTE** for the **Append**:

`WITH context AS`  
`(SELECT MAX("sequence_number") AS "max_seq" FROM "events"`  
`WHERE ((("event_type" = 'BookCopyAddedToCirculation') OR ("event_type" = 'BookCopyLentToReader') OR ("event_type" = 'BookCopyRemovedFromCirculation') OR ("event_type" = 'BookCopyReturnedByReader')) AND (payload @> '{"BookID": "0198226e-19f6-7d29-8be9-10871e23e820"}' OR payload @> '{"ReaderID": "0198226e-19f6-7d2a-8342-dc5c8d5a2cd5"}')))`  

The insert for the **Append** using the **CTE**:

`INSERT INTO "events" ("event_type", "payload")`  
`SELECT 'BookCopyAddedToCirculation', '{"BookID":"0198223b-12f8-74af-ada1-12ac0687b922","ISBN":"978-1-098-10013-1","Title":"Learning Domain-Driven Design","Authors":"Vlad Khononov","Edition":"First Edition","Publisher":"O''Reilly Media, Inc.","PublicationYear":2021}'`  
`FROM context" WHERE (COALESCE("max_seq", 0) = 6)`

If you look closely (I know it might be hard to read), you will notice that the where clause is the same in the **Query** and the **CTE**!  
At the point of the **Query** the highest sequence number of _**relevant events**_ was 6, so the **Append** guards this to be unchanged.



## Overview

This project provides a robust event store solution that enables:
- Event persistence and retrieval
- Stream-based event processing
- PostgreSQL integration for reliable storage
- Domain event handling for event-driven systems

## Features

- **PostgreSQL Backend**: Leverages PostgreSQL for reliable event storage
- **Event Filtering**: Built-in filtering capabilities for event queries
- **Domain Events**: Support for domain-driven design patterns
- **Benchmarking**: Performance testing utilities included
- **Docker Support**: Ready-to-use Docker Compose configuration

## Tech Stack

- **Language**: Go 1.24
- **Database**: PostgreSQL (latest - 17.5 at the time of this writing)
- **Key Dependencies**:
    - `github.com/jackc/pgx/v5` - PostgreSQL driver
    - `github.com/doug-martin/goqu/v9` - SQL query builder
    - `github.com/google/uuid` - UUID generation
    - `github.com/json-iterator/go` - Fast JSON marshaling
    - `github.com/stretchr/testify/assert` - Assertions for testing


## Quick Start

1. **Start PostgreSQL**: Use Docker Compose to run the database
   ```bash
   docker-compose up -d postgres_test
   ```

2. **Run Tests**: Execute the test suite
   ```bash
   go test ./...
   ```

3. **Benchmarks**: Run performance benchmarks
   ```bash
   go test -bench=. ./eventstore/engine/
   ```

## Docker Support

The project includes Docker Compose configuration with:
- **postgres_test**: Development database (port 5432)
- **postgres_benchmark**: Performance testing database (port 5433)

Both services include automatic database initialization from the `initdb/` directory.

## License

This project is licensed under the terms specified in `LICENSE.txt`.