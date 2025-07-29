# Development Guide

This guide covers setting up the development environment, running tests, and contributing to dynamic-streams-eventstore-go.

## Development Setup

### Prerequisites

- **Go 1.24+**
- **Docker & Docker Compose** (for test databases)
- **PostgreSQL 17+** (optional, for local development)

### Getting Started

1. **Clone the repository:**
```bash
git clone https://github.com/AntonStoeckl/dynamic-streams-eventstore-go.git
cd dynamic-streams-eventstore-go
```

2. **Install dependencies:**
```bash
go mod download
```

3. **Start test databases:**
```bash
# Start both test databases
docker-compose --file testutil/docker-compose.yml up -d

# Or start individually
docker-compose --file testutil/docker-compose.yml up -d postgres_test      # Port 5432
docker-compose --file testutil/docker-compose.yml up -d postgres_benchmark # Port 5433
```

4. **Verify setup:**
```bash
go test ./eventstore/postgresengine/
```

## Project Structure

```
├── eventstore/                           # Core event store implementation
│   ├── postgresengine/                  # PostgreSQL implementation
│   │   ├── postgres.go                  # Main implementation with adapters
│   │   ├── internal/adapters/           # Database adapter abstraction
│   │   │   ├── pgx_adapter.go          # pgx.Pool adapter
│   │   │   ├── sql_adapter.go          # database/sql adapter  
│   │   │   └── sqlx_adapter.go         # sqlx adapter
│   │   ├── postgres_test.go            # Functional tests
│   │   └── postgres_benchmark_test.go  # Performance benchmarks
│   ├── filter.go                       # Filter builder implementation
│   └── storable_event.go               # Event data structures
├── testutil/                           # Test infrastructure
│   ├── cmd/                            # Utility commands
│   │   ├── generate/                   # Fixture data generation
│   │   └── import/                     # Data import utilities
│   ├── initdb/                         # Database initialization
│   ├── helper/postgreswrapper/         # Adapter-agnostic test wrapper
│   ├── docker-compose.yml              # Test database setup
│   └── helper.go                       # Test utilities
├── example/                            # Example domain (used in tests)
│   ├── core/                           # Domain events and business logic
│   ├── shell/                          # Event mapping layer
│   └── config/                         # Test database configuration
├── docs/                               # Documentation
└── go.mod                              # Go module definition
```

## Running Tests

### Functional Tests

```bash
# Run all tests with default adapter (pgx.Pool)
go test ./...

# Test with specific database adapters
ADAPTER_TYPE=sqldb go test ./eventstore/postgresengine/   # database/sql
ADAPTER_TYPE=sqlx go test ./eventstore/postgresengine/    # sqlx

# Run tests with verbose output
go test -v ./eventstore/postgresengine/

# Run specific test
go test -v ./eventstore/postgresengine/ -run TestEventStore_Query
```

### Benchmark Tests

```bash
# Run all benchmarks with default adapter (pgx.Pool)
go test -bench=. ./eventstore/postgresengine/

# Test with specific database adapters
ADAPTER_TYPE=sqldb go test -bench=. ./eventstore/postgresengine/   # database/sql
ADAPTER_TYPE=sqlx go test -bench=. ./eventstore/postgresengine/    # sqlx

# Run specific benchmark
go test -bench=BenchmarkQuery ./eventstore/postgresengine/

# Compare adapter performance
go test -bench=. -count=3 ./eventstore/postgresengine/ > pgx_bench.txt
ADAPTER_TYPE=sqldb go test -bench=. -count=3 ./eventstore/postgresengine/ > sql_bench.txt
```

**Note:** Benchmarks require at least 10,000 fixture events. Use the fixture generation tools if needed.

### Test Coverage

```bash
# Generate coverage report
go test -coverprofile=coverage.out ./...

# View coverage in browser
go tool cover -html=coverage.out

# Get coverage percentage
go tool cover -func=coverage.out
```

## Fixture Data Management

For performance testing, you may need to generate fixture data:

### Generate Fixture Events

```bash
# Generate CSV file with fixture events
go run testutil/cmd/generate/generate_fixture_events_data.go

# This creates testutil/fixtures/events.csv
```

### Import Fixture Data

```bash
# Import CSV data into benchmark database
go run testutil/cmd/import/import_csv_data.go

# This imports data into the postgres_benchmark container
```

### Custom Fixture Generation

You can modify the generation parameters in `testutil/cmd/generate/generate_fixture_events_data.go`:

```go
// Adjust these values for your testing needs
const (
    numEvents = 100000        // Total events to generate
    numBooks = 1000          // Number of unique books  
    numReaders = 500         // Number of unique readers
)
```

## Code Style and Standards

### Formatting

```bash
# Format all code
go fmt ./...

# Check formatting
gofmt -d .
```

### Linting

```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run linter
golangci-lint run

# Run specific linters
golangci-lint run --enable=gosec,goconst
```

### Vetting

```bash
# Vet all packages
go vet ./...
```

## Dependency Management

```bash
# Add new dependency
go get github.com/some/package

# Update dependencies
go get -u ./...

# Tidy modules
go mod tidy

# Verify dependencies
go mod verify
```

## Database Development

### Schema Changes

The database schema is defined in `testutil/initdb/init.sql`. When making schema changes:

1. Update `init.sql`
2. Recreate test databases:
```bash
docker-compose --file testutil/docker-compose.yml down -v
docker-compose --file testutil/docker-compose.yml up -d
```

### Database Debugging

```bash
# Connect to test database
docker exec -it test_postgres_test_1 psql -U test -d eventstore

# Connect to benchmark database  
docker exec -it test_postgres_benchmark_1 psql -U test -d eventstore
```

Useful SQL queries for debugging:

```sql
-- Check event counts
SELECT COUNT(*) FROM events;

-- Check recent events
SELECT event_type, occurred_at, payload 
FROM events 
ORDER BY sequence_number DESC 
LIMIT 10;

-- Analyze query performance
EXPLAIN ANALYZE 
SELECT * FROM events 
WHERE payload @> '{"BookID": "some-id"}';

-- Check index usage
SELECT indexname, idx_tup_read, idx_tup_fetch 
FROM pg_stat_user_indexes 
WHERE relname = 'events';
```

## Debugging

### Enable Debug Logging

```go
// In your test code
import "log"

func TestDebugExample(t *testing.T) {
    // Enable verbose logging
    log.SetFlags(log.LstdFlags | log.Lshortfile)
    
    // Your test code
}
```

### SQL Query Logging

Enable PostgreSQL query logging in Docker:

```yaml
# In docker-compose.yml
services:
  postgres_test:
    command: postgres -c log_statement=all -c log_destination=stderr
```

### Common Issues

**Connection Issues:**
```bash
# Check if databases are running
docker-compose --file testutil/docker-compose.yml ps

# Check logs
docker-compose --file testutil/docker-compose.yml logs postgres_test
```

**Test Failures:**
```bash
# Clean state and retry
docker-compose --file testutil/docker-compose.yml down -v
docker-compose --file testutil/docker-compose.yml up -d
go test ./eventstore/postgresengine/
```

## Performance Profiling

### CPU Profiling

```bash
# Run benchmarks with CPU profiling
go test -bench=BenchmarkQuery -cpuprofile=cpu.prof ./eventstore/postgresengine/

# Analyze profile
go tool pprof cpu.prof
```

### Memory Profiling

```bash
# Run benchmarks with memory profiling
go test -bench=BenchmarkQuery -memprofile=mem.prof ./eventstore/postgresengine/

# Analyze profile
go tool pprof mem.prof
```

### Trace Analysis

```bash
# Generate execution trace
go test -bench=BenchmarkQuery -trace=trace.out ./eventstore/postgresengine/

# View trace
go tool trace trace.out
```

## Contributing

### Before Submitting PRs

1. **Run all tests:**
```bash
go test ./...
```

2. **Run benchmarks:**
```bash
go test -bench=. ./eventstore/postgresengine/
```

3. **Check formatting:**
```bash
go fmt ./...
```

4. **Run linter:**
```bash
golangci-lint run
```

5. **Update documentation** if needed

### Commit Guidelines

- Use clear, descriptive commit messages
- Reference issue numbers when applicable
- Keep commits focused and atomic

### Testing Guidelines

- Add tests for new functionality
- Maintain or improve test coverage
- Include both positive and negative test cases
- Add benchmarks for performance-critical changes

## Build and Release

### Building

```bash
# Build all packages
go build ./...

# Build specific package
go build ./eventstore/postgresengine/

# Check for build issues
go build -v ./...
```

### Module Publishing

This project follows semantic versioning. When ready to release:

1. **Tag the release:**
```bash
git tag v1.2.3
git push origin v1.2.3
```

2. **Go modules automatically pick up the tag**

### Documentation Updates

When making changes, update relevant documentation:

- Update `CLAUDE.md` for development guidance
- Update `docs/` files for user-facing changes
- Update `README.md` if necessary
- Update code comments and examples

## Development Tools

### Recommended VS Code Extensions

- Go (official Go extension)
- PostgreSQL (syntax highlighting)
- Docker (container management)
- GitLens (git integration)

### Recommended IntelliJ/GoLand Plugins

- PostgreSQL integration
- Docker integration  
- Go modules support

### CLI Tools

```bash
# Install useful Go tools
go install golang.org/x/tools/cmd/goimports@latest
go install golang.org/x/tools/cmd/godoc@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```