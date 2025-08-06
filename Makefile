# Makefile for dynamic-streams-eventstore-go

.PHONY: help install-tools test test-verbose test-coverage lint fmt clean
.PHONY: test-generic test-factory test-observability test-pgx test-sql test-sqlx test-all-adapters
.PHONY: benchmark benchmark-pgx benchmark-sql benchmark-sqlx benchmark-all
.PHONY: build-fixtures run-generate run-import fixtures-generate fixtures-import

# Default target
help:
	@echo "Available targets:"
	@echo "  install-tools       Install/update development tools (golangci-lint)"
	@echo ""
	@echo "Testing:"
	@echo "  test               Run all tests (generic + pgx adapter)"
	@echo "  test-verbose       Run tests with verbose output"
	@echo "  test-coverage      Generate test coverage report"
	@echo "  test-generic       Run generic tests only (factory + observability)"
	@echo "  test-factory       Run factory function tests only"
	@echo "  test-observability Run observability tests only"
	@echo "  test-pgx           Run functional tests with pgx adapter"
	@echo "  test-sql           Run functional tests with sql.DB adapter"
	@echo "  test-sqlx          Run functional tests with sqlx adapter"
	@echo "  test-all-adapters  Run functional tests with all 3 adapters"
	@echo ""
	@echo "Benchmarks:"
	@echo "  benchmark          Run benchmarks (pgx adapter)"
	@echo "  benchmark-pgx      Run benchmarks with pgx adapter"
	@echo "  benchmark-sql      Run benchmarks with sql.DB adapter"
	@echo "  benchmark-sqlx     Run benchmarks with sqlx adapter"
	@echo "  benchmark-all      Run benchmarks with all 3 adapters"
	@echo ""
	@echo "Fixture Management:"
	@echo "  build-fixtures     Build fixture generation and import tools"
	@echo "  run-generate       Generate fixture events (build if needed)"
	@echo "  run-import         Import CSV data (build if needed)"
	@echo "  fixtures-generate  Alias for run-generate"
	@echo "  fixtures-import    Alias for run-import"
	@echo ""
	@echo "Code Quality:"
	@echo "  lint               Run golangci-lint on all code"
	@echo "  fmt                Format code with go fmt"
	@echo "  clean              Clean up generated files"

# Install/update development tools
install-tools:
	@echo "Installing/updating development tools..."
	@echo "Installing golangci-lint..."
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin
	@echo "Verifying installation..."
	@golangci-lint version
	@echo "✅ Development tools installed successfully"

# Test targets
test:
	@echo "Running tests..."
	@go test ./...

test-verbose:
	@echo "Running tests with verbose output..."
	@go test -v ./...

test-coverage:
	@echo "Generating test coverage report..."
	@go test -coverprofile=coverage.out ./...
	@echo "Coverage report generated: coverage.out"
	@echo "View in browser with: go tool cover -html=coverage.out"

# Generic tests (adapter-independent factory and observability tests)
test-generic:
	@echo "Running generic tests (adapter-independent)..."
	@go test ./eventstore/postgresengine/ -run "^Test_FactoryFunctions_|^Test_Observability_"

# Factory function tests (adapter-independent)
test-factory:
	@echo "Running factory function tests..."
	@go test ./eventstore/postgresengine/ -run "^Test_FactoryFunctions_"

# Observability tests (adapter-independent)
test-observability:
	@echo "Running observability tests..."
	@go test ./eventstore/postgresengine/ -run "^Test_Observability_"

# Adapter-specific functional tests (skip generic tests)
test-pgx:
	@echo "Running functional tests with pgx.Pool adapter..."
	@ADAPTER_TYPE=pgx.pool go test ./eventstore/postgresengine/ -run "^Test_" -skip "^Test_FactoryFunctions_|^Test_Observability_"

test-sql:
	@echo "Running functional tests with database/sql adapter..."
	@ADAPTER_TYPE=sql.db go test ./eventstore/postgresengine/ -run "^Test_" -skip "^Test_FactoryFunctions_|^Test_Observability_"

test-sqlx:
	@echo "Running functional tests with sqlx adapter..."
	@ADAPTER_TYPE=sqlx.db go test ./eventstore/postgresengine/ -run "^Test_" -skip "^Test_FactoryFunctions_|^Test_Observability_"

test-all-adapters:
	@echo "Running functional tests with all database adapters..."
	@echo ""
	@echo "🔹 Testing with pgx.Pool adapter:"
	@ADAPTER_TYPE=pgx.pool go test ./eventstore/postgresengine/ -run "^Test_" -skip "^Test_FactoryFunctions_|^Test_Observability_"
	@echo ""
	@echo "🔹 Testing with database/sql adapter:"
	@ADAPTER_TYPE=sql.db go test ./eventstore/postgresengine/ -run "^Test_" -skip "^Test_FactoryFunctions_|^Test_Observability_"
	@echo ""
	@echo "🔹 Testing with sqlx adapter:"
	@ADAPTER_TYPE=sqlx.db go test ./eventstore/postgresengine/ -run "^Test_" -skip "^Test_FactoryFunctions_|^Test_Observability_"
	@echo ""
	@echo "✅ All adapter tests complete"

# Benchmark targets
benchmark:
	@echo "Running benchmarks with pgx adapter (default)..."
	@go test -bench=. ./eventstore/postgresengine/

benchmark-pgx:
	@echo "Running benchmarks with pgx.Pool adapter..."
	@ADAPTER_TYPE=pgx.pool go test -bench=. ./eventstore/postgresengine/

benchmark-sql:
	@echo "Running benchmarks with database/sql adapter..."
	@ADAPTER_TYPE=sql.db go test -bench=. ./eventstore/postgresengine/

benchmark-sqlx:
	@echo "Running benchmarks with sqlx adapter..."
	@ADAPTER_TYPE=sqlx.db go test -bench=. ./eventstore/postgresengine/

benchmark-all:
	@echo "Running benchmarks with all database adapters..."
	@echo ""
	@echo "🔹 Benchmarking with pgx.Pool adapter:"
	@ADAPTER_TYPE=pgx.pool go test -bench=. ./eventstore/postgresengine/
	@echo ""
	@echo "🔹 Benchmarking with database/sql adapter:"
	@ADAPTER_TYPE=sql.db go test -bench=. ./eventstore/postgresengine/
	@echo ""
	@echo "🔹 Benchmarking with sqlx adapter:"
	@ADAPTER_TYPE=sqlx.db go test -bench=. ./eventstore/postgresengine/
	@echo ""
	@echo "✅ Benchmark comparison complete"

# Fixture management targets
GENERATE_DIR = testutil/postgresengine/cmd/generate
IMPORT_DIR = testutil/postgresengine/cmd/import
GENERATE_BIN = $(GENERATE_DIR)/generate_fixture_events_data
IMPORT_BIN = $(IMPORT_DIR)/import_csv_data

build-fixtures:
	@echo "Building fixture generation and import tools..."
	@echo "Building generate_fixture_events_data..."
	@go build -o $(GENERATE_BIN) ./$(GENERATE_DIR)
	@echo "Building import_csv_data..."
	@go build -o $(IMPORT_BIN) ./$(IMPORT_DIR)
	@echo "✅ Fixture tools built successfully"

run-generate: 
	@echo "Generating fixture events..."
	@if [ -f $(GENERATE_BIN) ]; then \
		echo "Using compiled binary: $(GENERATE_BIN)"; \
		./$(GENERATE_BIN); \
	else \
		echo "Compiled binary not found, building and running..."; \
		go build -o $(GENERATE_BIN) ./$(GENERATE_DIR) && ./$(GENERATE_BIN); \
	fi
	@echo "✅ Fixture generation complete"

run-import:
	@echo "Importing CSV data..."
	@if [ -f $(IMPORT_BIN) ]; then \
		echo "Using compiled binary: $(IMPORT_BIN)"; \
		./$(IMPORT_BIN); \
	else \
		echo "Compiled binary not found, building and running..."; \
		go build -o $(IMPORT_BIN) ./$(IMPORT_DIR) && ./$(IMPORT_BIN); \
	fi
	@echo "✅ CSV import complete"

# Aliases for fixture management
fixtures-generate: run-generate
fixtures-import: run-import

# Code quality targets
lint:
	@echo "Running golangci-lint on all code..."
	@golangci-lint run
	@echo "✅ Linting complete"


fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Cleanup
clean:
	@echo "Cleaning up..."
	@rm -f coverage.out
	@rm -f cpu.prof mem.prof trace.out
	@rm -f $(GENERATE_BIN) $(IMPORT_BIN)
	@echo "✅ Cleanup complete"