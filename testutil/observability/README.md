# EventStore Observability Stack Integration

This directory provides a complete observability stack for testing and demonstrating the EventStore's OpenTelemetry integration with real backends.

## Stack Components

- **Prometheus** (localhost:9090)—Metrics storage and query engine
- **Grafana** (localhost:3000)—Visualization dashboards  
- **Jaeger** (localhost:16686)—Distributed tracing UI (OTLP: localhost:4319)
- **OpenTelemetry Collector** (localhost:4317)—Metrics routing to Prometheus
- **PostgreSQL** (localhost:5433)—Uses an existing benchmark database with fixture data

**Note**: Traces are sent directly to Jaeger's OTLP endpoint, while metrics flow through the OTEL Collector to Prometheus.

## Quick Start

### 1. Start the Observability Stack

```bash
cd testutil/observability
docker compose up -d
```

### 2. Run the Integration Test

```bash
# From project root
OBSERVABILITY_ENABLED=true go test -run Test_Observability_Eventstore_WithRealObservabilityStack -v ./eventstore/postgresengine/
```

### 3. View Observability Data

- **Grafana Dashboard**: http://localhost:3000 (admin/admin)
  - Pre-configured with EventStore test load dashboard
  - Real-time metrics from test execution
- **Jaeger Traces**: http://localhost:16686
  - Distributed traces showing operation spans
  - Detailed timing and error information
- **Prometheus Metrics**: http://localhost:9090
  - Raw metrics query interface
  - Custom PromQL queries

### 4. Cleanup

```bash
docker compose down
```

## Test Details

The integration test `Test_Observability_Eventstore_WithRealObservabilityStack_RealisticLoad`:

- Uses real OpenTelemetry providers (not test spies)
- Connects to the existing benchmark PostgreSQL database (localhost:5433)
- Leverages existing fixture data for realistic load patterns
- Generates real metrics, traces, and logs visible in the observability backends
- Runs only when `OBSERVABILITY_ENABLED=true` environment variable is set

## Dashboard Features

### EventStore Test Load Dashboard

- **Operation Duration**: P50, P95, P99 percentiles for query and append operations
- **Operations Rate**: Real-time operations per second
- **Error Rates**: Database errors and concurrency conflicts
- **Events Processed**: Total counters for queried and appended events

## Configuration

- **Prometheus**: Scrapes metrics from OpenTelemetry Collector every 15s
- **Grafana**: Auto-provisioned with datasources and dashboards
- **Jaeger**: Receives traces via OpenTelemetry Collector
- **OTEL Collector**: Routes telemetry to appropriate backends

## Integration with Test Infrastructure

This observability stack integrates seamlessly with the existing test infrastructure:

- Uses `CreateWrapperWithBenchmarkConfig()` for database connections
- Follows the same patterns as existing observability tests
- Compatible with existing fixture management and test helpers
- Optional execution prevents interference with normal testing workflows

## Production Relevance

This setup demonstrates how to deploy EventStore observability in production environments:

- Standard observability stack components
- Production-ready OpenTelemetry configuration
- Real performance metrics and traces
- Error monitoring and alerting patterns