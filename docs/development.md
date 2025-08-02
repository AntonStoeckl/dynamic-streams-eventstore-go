# Development Guide

This guide covers setting up the development environment, running tests, and contributing to dynamic-streams-eventstore-go.

## Development Setup

### Prerequisites

- **Go 1.24+**
- **Docker & Docker Compose** (for test databases)
- **PostgreSQL 17+** (optional, for local development)
- **golangci-lint** (for code quality checks)

### Getting Started

1. **Clone the repository:**
```bash
git clone https://github.com/AntonStoeckl/dynamic-streams-eventstore-go.git
cd dynamic-streams-eventstore-go
```

2. **Install development tools:**
```bash
# Install all development tools (recommended)
make install-tools

# Or install golangci-lint manually:
# go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
# curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin
```

3. **Install and update dependencies:**
```bash
go mod tidy
```

4. **Start test databases:**
```bash
# Start both test databases
docker-compose --file testutil/postgresengine/docker-compose.yml up -d

# Or start individually
docker-compose --file testutil/postgresengine/docker-compose.yml up -d postgres_test      # Port 5432
docker-compose --file testutil/postgresengine/docker-compose.yml up -d postgres_benchmark # Port 5433
```

5. **Verify setup:**
```bash
# Run tests to verify everything works
make test

# Or run tests manually
go test ./eventstore/postgresengine/
```

### Development Workflow

The project includes a `Makefile` with common development tasks:

```bash
# Show all available targets
make help

# Install/update development tools
make install-tools

# Run tests
make test                # Run all tests
make test-verbose        # Run with verbose output
make test-coverage       # Generate coverage report

# Code quality
make lint                # Run golangci-lint
make fmt                 # Format code

# Cleanup
make clean               # Remove generated files
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
│   │   ├── postgres_test.go            # Functional tests (adapter-dependent)
│   │   ├── postgres_factory_test.go    # Factory function tests (adapter-independent)
│   │   ├── postgres_observability_test.go # Observability tests (adapter-independent)
│   │   └── postgres_benchmark_test.go  # Performance benchmarks
│   ├── filter.go                       # Filter builder implementation
│   └── storable_event.go               # Event data structures
├── testutil/                           # Test infrastructure
│   └── postgresengine/                 # PostgreSQL-specific test utilities
│       ├── cmd/                        # Utility commands
│       │   ├── generate/               # Fixture data generation
│       │   └── import/                 # Data import utilities
│       ├── initdb/                     # Database initialization
│       ├── helper/postgreswrapper/     # Adapter-agnostic test wrapper
│       ├── docker-compose.yml          # Test database setup
│       ├── fixtures/                   # Generated fixture data
│       └── helper.go                   # Test utilities
├── example/                            # Example domain (used in tests)
│   ├── shared/                         # Shared components
│   │   ├── core/                       # Domain events (domain layer)
│   │   └── shell/                      # Event mapping (infrastructure layer)
│   │       └── config/                 # Test database configuration
│   └── features/                       # Feature implementations
│       └── removebookcopy/             # Complete feature slice example
├── docs/                               # Documentation
└── go.mod                              # Go module definition
```

## Running Tests

### Functional Tests

```bash
# Run all tests with default adapter (pgx.Pool)
go test ./...

# Run factory function tests (adapter-independent)
go test ./eventstore/postgresengine/ -run "^Test_FactoryFunctions_"

# Run observability tests (adapter-independent)  
go test ./eventstore/postgresengine/ -run "^Test_Observability_"

# Run all generic tests (factory + observability)
go test ./eventstore/postgresengine/ -run "^Test_FactoryFunctions_|^Test_Observability_"

# Run functional tests with specific database adapters
go test ./eventstore/postgresengine/ -run "^Test_" -skip "^Test_FactoryFunctions_|^Test_Observability_"                    # pgx.Pool (default)
ADAPTER_TYPE=sql.db go test ./eventstore/postgresengine/ -run "^Test_" -skip "^Test_FactoryFunctions_|^Test_Observability_"  # database/sql
ADAPTER_TYPE=sqlx.db go test ./eventstore/postgresengine/ -run "^Test_" -skip "^Test_FactoryFunctions_|^Test_Observability_" # sqlx

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
ADAPTER_TYPE=sql.db go test -bench=. ./eventstore/postgresengine/   # database/sql
ADAPTER_TYPE=sqlx.db go test -bench=. ./eventstore/postgresengine/  # sqlx

# Run specific benchmark
go test -bench=BenchmarkQuery ./eventstore/postgresengine/

# Compare adapter performance
go test -bench=. -count=3 ./eventstore/postgresengine/ > pgx_bench.txt
ADAPTER_TYPE=sql.db go test -bench=. -count=3 ./eventstore/postgresengine/ > sql_bench.txt
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

For performance testing, generate fixture data and import it into the benchmark database.

**Import Speed (fastest to slowest):**
1. **CSV server-side import** (recommended) — Fast, mounted into Docker container
2. **CSV local import** — 5x slower, sometimes fails with millions of events  
3. **SQL file import** — Very slow, only suitable for small datasets

### Generate Fixture Events

```bash
# Generate CSV file with fixture events
go run testutil/postgresengine/cmd/generate/generate_fixture_events_data.go

# This creates testutil/postgresengine/fixtures/events.csv

# After creating fixtures, restart containers to mount the new fixture file into a volume
docker-compose --file testutil/docker-compose.yml down
docker-compose --file testutil/postgresengine/docker-compose.yml up -d
```

### Import Fixture Data

```bash
# Import CSV data into benchmark database
go run testutil/postgresengine/cmd/import/import_csv_data.go
```

### Custom Fixture Generation

You can modify the generation parameters in `testutil/postgresengine/cmd/generate/generate_fixture_events_data.go`:

```go
// Adjust these values for your testing needs
const (
    // Number of "Something has happened" events to be created
    NumSomethingHappenedEvents = 9 * million // Default: 9 million events
    
    // Number of "BookCopy..." events to be created  
    NumBookCopyEvents = 1 * million          // Default: 1 million events
    
    // Total events: 10 million (generates ~3.9GB CSV/SQL files)
    
    // Control output formats
    WriteCSVFileEnabled = true  // Generate CSV file (recommended)
    WriteSQLFileEnabled = false // Generate SQL file (slower import)
)
```

**Warning:** 10 million fixture events create ~3.9GB files. Generation takes about 25 seconds, and importing takes about 4 minutes. Use smaller values for faster fixture loading.

## Common Issues

**Connection Issues:**
```bash
# Check if databases are running
docker-compose --file testutil/postgresengine/docker-compose.yml ps

# Check logs
docker-compose --file testutil/postgresengine/docker-compose.yml logs postgres_test
```

**Test Failures:**
```bash
# Clean state and retry
docker-compose --file testutil/postgresengine/docker-compose.yml down -v
docker-compose --file testutil/postgresengine/docker-compose.yml up -d
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

**Note:** I generally like filed issues more than pull requests for discussing changes and improvements.

### Before Submitting PRs

1. **Run all tests:**
```bash
make test
# Or: go test ./...
```

2. **Run benchmarks:**
```bash
go test -bench=. ./eventstore/postgresengine/
```

3. **Check formatting:**
```bash
make fmt
# Or: go fmt ./...
```

4. **Run linter:**
```bash
make lint
# Or: golangci-lint run
```

**Advanced linting options:**
```bash
# Run with verbose output
golangci-lint run -v

# Run only specific linters
golangci-lint run --enable=errcheck,govet,staticcheck

# Auto-fix issues (if supported by the linter)
golangci-lint run --fix
```

The project uses a comprehensive `.golangci.yml` configuration with 25+ linters enabled including:
- **Core quality**: errcheck, govet, staticcheck, unused
- **Security**: gosec (security analyzer)
- **Performance**: prealloc, unconvert
- **Best practices**: goconst, contextcheck, errorlint, sqlclosecheck
- **Code style**: revive, gocritic, misspell

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

### Documentation Updates

When making changes, update the relevant documentation:

- Update `docs/` files for user-facing changes
- Update `README.md` if necessary
- Update code comments and examples

