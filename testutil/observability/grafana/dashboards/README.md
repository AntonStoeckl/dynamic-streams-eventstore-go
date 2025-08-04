# EventStore Load Generator Dashboard

This directory contains Grafana dashboards for monitoring EventStore performance and load generation.

## Dashboards

### eventstore-load-generator.json
A comprehensive dashboard specifically designed for monitoring the EventStore Load Generator with the following panels:

#### Load Generator Performance
- **Request Rate**: Real-time requests/second with target rate comparison
- **Scenario Distribution**: Pie chart showing circulation (4%), lending (94%), error (2%) breakdown  
- **Success vs Error Rate**: Timeline showing successful operations vs errors
- **Request Duration**: P50, P95, P99 percentiles for load generator operations

#### EventStore Operations
- **Operation Duration**: P50, P95, P99 percentiles for Query and Append operations
- **Operations Rate**: Events queried and appended per second
- **Error Breakdown**: Categorized error types (concurrency conflicts, query errors, etc.)

#### Business Logic Monitoring
- **Scenario Execution**: Timeline showing circulation, lending, and error scenario execution
- **Concurrency Conflicts**: Rate of optimistic concurrency control failures
- **Performance Summary**: Table with key metrics overview

### eventstore-test-load.json  
Pre-existing dashboard for general EventStore test load monitoring.

## Metrics Used

### Load Generator Metrics
- `load_generator_current_rate` - Current request rate (gauge)
- `load_generator_request_duration_seconds` - Request execution duration (histogram)
- `load_generator_scenarios_total` - Total scenarios executed by type (counter)
- `load_generator_errors_total` - Total errors by type and scenario (counter)

### EventStore Metrics
- `eventstore_query_duration_seconds` - Query operation duration (histogram)
- `eventstore_append_duration_seconds` - Append operation duration (histogram)  
- `eventstore_events_queried_total` - Total events queried (counter)
- `eventstore_events_appended_total` - Total events appended (counter)

## Auto-Provisioning

Dashboards are automatically loaded when the observability stack starts via:
- Volume mount: `./grafana/dashboards:/var/lib/grafana/dashboards`
- Provisioning config: `./grafana/provisioning/dashboards/dashboards.yml`

## Usage

1. Start the observability stack:
   ```bash
   cd testutil/observability && docker compose up -d
   ```

2. Start the benchmark PostgreSQL database:
   ```bash
   cd testutil/postgresengine && docker compose up -d postgres_benchmark
   ```

3. Run the load generator with observability:
   ```bash
   cd example/demo/cmd/load-generator
   ./load-generator --observability-enabled=true --rate=30
   ```

4. Access the dashboard:
   - Grafana UI: http://localhost:3000 (admin/admin)
   - Navigate to "EventStore" folder → "EventStore Load Generator Dashboard"

## Dashboard Features

- **Real-time Updates**: 5-second refresh for live monitoring
- **Time Range**: Default 15-minute window with quick selectors (5m, 30m, 1h, 3h)
- **Color Coding**: Thresholds for performance indicators
- **Interactive Legends**: Click to filter metrics
- **Responsive Layout**: Panels automatically adjust to screen size

## Expected Behavior

When load generator is running with observability enabled, you should see:
- Request rate matching configured target (default 30 req/s)
- Scenario distribution following 4,94,2 pattern (circulation, lending, errors)
- Occasional concurrency conflicts (expected behavior)
- Sub-second operation durations for most scenarios
- EventStore operations correlated with load generator activity