## Investigate Duration Metric Values in Observability
- **Created**: 2025-08-06
- **Priority**: High - Critical for accurate performance monitoring
- **Objective**: Debug inconsistency between duration metrics reported in Grafana vs actual benchmark test performance

### Current Problem Analysis
- **Grafana metrics**: Showing different performance values than expected
- **Benchmark tests**: Known performance characteristics (~2.5ms per operation)
- **Disconnect**: Duration metrics in observability don't correlate with benchmark results
- **Monitoring concern**: Cannot trust production performance monitoring if metrics are inaccurate

### Files/Packages to Review in Next Session
1. **Metrics Collection**:
   - `eventstore/postgresengine/observability.go` (duration recording logic)
   - `eventstore/postgresengine/postgres.go` (timer start/stop placement)
   - `eventstore/oteladapters/metrics_collector.go` (OpenTelemetry duration recording)

2. **Prometheus/OTEL Pipeline**:
   - `testutil/observability/otel-collector-config.yml` (metric processing configuration)
   - Grafana dashboard queries: Check if rate() calculations are correct
   - Histogram bucket configuration and percentile calculations

3. **Test Infrastructure**:
   - `eventstore/postgresengine/postgres_benchmark_test.go` (baseline performance measurements)
   - `postgres_observability_test.go` (compare actual vs observed durations)
   - Load generator duration reporting vs actual operation times

### Next Session Investigation Plan
1. **Baseline Verification**: Run benchmark tests to establish known performance baseline
2. **Instrumentation Audit**: Verify timer placement around actual database operations (not including observability overhead)
3. **Metrics Pipeline Debug**: Check OpenTelemetry histogram recording and Prometheus ingestion
4. **Dashboard Query Analysis**: Review Grafana queries for duration calculations (rate() vs histogram_quantile())
5. **Load Generator Comparison**: Compare load generator reported durations with EventStore metrics
6. **Unit Conversion Check**: Verify seconds vs milliseconds consistency across the pipeline

### Potential Root Causes
- **Timer Placement**: Timing includes observability overhead instead of just database operations
- **Unit Conversion**: Metrics recorded in nanoseconds but displayed assuming milliseconds
- **Aggregation Issues**: Histogram buckets or percentile calculations configured incorrectly
- **Pipeline Latency**: OpenTelemetry collector introduces delays in metric reporting
- **Dashboard Queries**: Grafana queries using wrong rate windows or aggregation functions

### Success Criteria
- Duration metrics match benchmark test measurements within ±10%
- Grafana dashboard shows realistic performance values (2-5ms range for typical operations)
- Clear documentation of what each duration metric measures
- Validated metrics pipeline from EventStore through to Grafana display
- Reliable performance monitoring for production usage

---
