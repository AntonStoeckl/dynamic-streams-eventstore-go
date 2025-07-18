# Dynamic Streams EventStore

A Go-based event store implementation with PostgreSQL backend designed for event-driven architectures and stream processing.

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