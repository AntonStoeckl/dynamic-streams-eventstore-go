# Makefile for dynamic-streams-eventstore-go

.PHONY: help install-tools test test-quick test-verbose test-coverage lint fmt clean
.PHONY: test-generic test-factory test-observability test-pgx test-sql test-sqlx test-all-adapters
.PHONY: benchmark benchmark-pgx benchmark-sql benchmark-sqlx benchmark-all
.PHONY: restart-benchmark-db restart-test-db restart-observability-stack
.PHONY: stop-benchmark-db stop-test-db stop-observability-stack

# Default target
help:
	@echo "Available targets:"
	@echo "  install-tools       Install/update development tools (golangci-lint)"
	@echo ""
	@echo "Testing:"
	@echo "  test               Run ALL tests (all packages + all 3 adapters)"
	@echo "  test-quick         Run quick tests (all packages + pgx adapter only)"
	@echo "  test-verbose       Run ALL tests with verbose output (all adapters)"
	@echo "  test-coverage      Generate comprehensive coverage (all adapters)"
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
	@echo "Container Management:"
	@echo "  restart-benchmark-db       Restart benchmark database stack (master + replica)"
	@echo "  restart-test-db           Restart test database"
	@echo "  restart-observability-stack  Restart observability stack (Grafana, Prometheus, Jaeger)"
	@echo "  stop-benchmark-db         Stop benchmark database stack"
	@echo "  stop-test-db              Stop test database"
	@echo "  stop-observability-stack  Stop observability stack"
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
	@echo "Running tests (default: all packages + all adapters)..."
	@echo ""
	@echo "🔹 Running generic tests (adapter-independent)..."
	@go test ./eventstore/postgresengine/ -run "^Test_FactoryFunctions_|^Test_Observability_"
	@echo ""
	@echo "🔹 Running main eventstore package tests..."
	@go test ./eventstore/ -v
	@echo ""
	@echo "🔹 Running oteladapters tests..."
	@go test ./eventstore/oteladapters/ -v
	@echo ""
	@echo "🔹 Running functional tests with pgx.Pool adapter:"
	@ADAPTER_TYPE=pgx.pool go test ./eventstore/postgresengine/ -run "^Test_" -skip "^Test_FactoryFunctions_|^Test_Observability_"
	@echo ""
	@echo "🔹 Running functional tests with database/sql adapter:"
	@ADAPTER_TYPE=sql.db go test ./eventstore/postgresengine/ -run "^Test_" -skip "^Test_FactoryFunctions_|^Test_Observability_"
	@echo ""
	@echo "🔹 Running functional tests with sqlx adapter:"
	@ADAPTER_TYPE=sqlx.db go test ./eventstore/postgresengine/ -run "^Test_" -skip "^Test_FactoryFunctions_|^Test_Observability_"
	@echo ""
	@echo "✅ All tests complete"

test-verbose:
	@echo "Running tests with verbose output (all packages + all adapters)..."
	@echo ""
	@echo "🔹 Running generic tests (adapter-independent)..."
	@go test -v ./eventstore/postgresengine/ -run "^Test_FactoryFunctions_|^Test_Observability_"
	@echo ""
	@echo "🔹 Running main eventstore package tests..."
	@go test -v ./eventstore/
	@echo ""
	@echo "🔹 Running oteladapters tests..."
	@go test -v ./eventstore/oteladapters/
	@echo ""
	@echo "🔹 Running functional tests with pgx.Pool adapter:"
	@ADAPTER_TYPE=pgx.pool go test -v ./eventstore/postgresengine/ -run "^Test_" -skip "^Test_FactoryFunctions_|^Test_Observability_"
	@echo ""
	@echo "🔹 Running functional tests with database/sql adapter:"
	@ADAPTER_TYPE=sql.db go test -v ./eventstore/postgresengine/ -run "^Test_" -skip "^Test_FactoryFunctions_|^Test_Observability_"
	@echo ""
	@echo "🔹 Running functional tests with sqlx adapter:"
	@ADAPTER_TYPE=sqlx.db go test -v ./eventstore/postgresengine/ -run "^Test_" -skip "^Test_FactoryFunctions_|^Test_Observability_"
	@echo ""
	@echo "✅ All verbose tests complete"

test-coverage:
	@echo "Generating comprehensive test coverage report (all adapters)..."
	@echo ""
	@echo "🔹 Running coverage with generic tests..."
	@go test -coverprofile=coverage-generic.out -coverpkg=./eventstore/... -covermode=atomic ./eventstore/postgresengine/ -run "^Test_FactoryFunctions_|^Test_Observability_"
	@echo ""
	@echo "🔹 Running coverage for main eventstore package..."
	@go test -coverprofile=coverage-eventstore.out -coverpkg=./eventstore/... -covermode=atomic ./eventstore/
	@echo ""
	@echo "🔹 Running coverage for oteladapters package..."
	@go test -coverprofile=coverage-oteladapters.out -coverpkg=./eventstore/... -covermode=atomic ./eventstore/oteladapters/
	@echo ""
	@echo "🔹 Running coverage with pgx.Pool adapter:"
	@ADAPTER_TYPE=pgx.pool go test -coverprofile=coverage-pgx.out -coverpkg=./eventstore/... -covermode=atomic ./eventstore/postgresengine/ -run "^Test_" -skip "^Test_FactoryFunctions_|^Test_Observability_"
	@echo ""
	@echo "🔹 Running coverage with database/sql adapter:"
	@ADAPTER_TYPE=sql.db go test -coverprofile=coverage-sql.out -coverpkg=./eventstore/... -covermode=atomic ./eventstore/postgresengine/ -run "^Test_" -skip "^Test_FactoryFunctions_|^Test_Observability_"
	@echo ""
	@echo "🔹 Running coverage with sqlx adapter:"
	@ADAPTER_TYPE=sqlx.db go test -coverprofile=coverage-sqlx.out -coverpkg=./eventstore/... -covermode=atomic ./eventstore/postgresengine/ -run "^Test_" -skip "^Test_FactoryFunctions_|^Test_Observability_"
	@echo ""
	@echo "🔹 Merging coverage profiles..."
	@echo "mode: atomic" > coverage.out
	@grep -v "mode:" coverage-generic.out coverage-eventstore.out coverage-oteladapters.out coverage-pgx.out coverage-sql.out coverage-sqlx.out >> coverage.out 2>/dev/null || true
	@echo "✅ Comprehensive coverage report generated: coverage.out"
	@echo "View in browser with: go tool cover -html=coverage.out"

# Quick test target (single adapter for faster feedback)
test-quick:
	@echo "Running quick tests (pgx adapter only)..."
	@echo ""
	@echo "🔹 Running generic tests (adapter-independent)..."
	@go test ./eventstore/postgresengine/ -run "^Test_FactoryFunctions_|^Test_Observability_"
	@echo ""
	@echo "🔹 Running main eventstore package tests..."
	@go test ./eventstore/
	@echo ""
	@echo "🔹 Running oteladapters tests..."
	@go test ./eventstore/oteladapters/
	@echo ""
	@echo "🔹 Running functional tests with pgx.Pool adapter:"
	@ADAPTER_TYPE=pgx.pool go test ./eventstore/postgresengine/ -run "^Test_" -skip "^Test_FactoryFunctions_|^Test_Observability_"
	@echo ""
	@echo "✅ Quick tests complete"

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
	@rm -f coverage.out coverage-*.out
	@rm -f cpu.prof mem.prof trace.out
	@echo "✅ Cleanup complete"

# Container management targets
restart-benchmark-db:
	@echo "🔄 Restarting benchmark database stack (master + replica)..."
	@docker compose -f testutil/postgresengine/docker-compose.yml down postgres_benchmark_master postgres_benchmark_replica > /dev/null 2>&1
	@docker compose -f testutil/postgresengine/docker-compose.yml up -d postgres_benchmark_master postgres_benchmark_replica > /dev/null 2>&1
	@echo "✅ Benchmark database stack restarted (master: localhost:5433, replica: localhost:5434)"

restart-test-db:
	@echo "🔄 Restarting test database..."
	@docker compose -f testutil/postgresengine/docker-compose.yml down postgres_test > /dev/null 2>&1
	@docker compose -f testutil/postgresengine/docker-compose.yml up -d postgres_test > /dev/null 2>&1
	@echo "✅ Test database restarted (localhost:5432)"

restart-observability-stack:
	@echo "🔄 Restarting observability stack..."
	@docker compose -f testutil/observability/docker-compose.yml down > /dev/null 2>&1
	@docker compose -f testutil/observability/docker-compose.yml up -d > /dev/null 2>&1
	@echo "✅ Observability stack restarted:"
	@echo "   - Grafana: http://localhost:3000 (admin/secretpw)"
	@echo "   - Prometheus: http://localhost:9090"
	@echo "   - Jaeger: http://localhost:16686"

stop-benchmark-db:
	@echo "🛑 Stopping benchmark database stack (master + replica)..."
	@docker compose -f testutil/postgresengine/docker-compose.yml down postgres_benchmark_master postgres_benchmark_replica > /dev/null 2>&1
	@echo "✅ Benchmark database stack stopped"

stop-test-db:
	@echo "🛑 Stopping test database..."
	@docker compose -f testutil/postgresengine/docker-compose.yml down postgres_test > /dev/null 2>&1
	@echo "✅ Test database stopped"

stop-observability-stack:
	@echo "🛑 Stopping observability stack..."
	@docker compose -f testutil/observability/docker-compose.yml down > /dev/null 2>&1
	@echo "✅ Observability stack stopped"