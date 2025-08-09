# Ready to Implement

This file tracks larger plans that are ready for implementation in the Dynamic Event Streams EventStore project.

## Implement Retry Logic for Command/Query Handlers
- **Created**: 2025-08-06
- **Priority**: Medium - Production resilience improvement
- **Objective**: Add configurable retry logic with exponential backoff to command and query handlers for handling transient database errors and concurrency conflicts

### Current Problem Analysis
- **Current behavior**: Single-attempt operations fail immediately on transient errors (network issues, temporary DB unavailability, high concurrency)
- **Production impact**: Reduces system resilience under load or during infrastructure issues
- **User experience**: Operations that could succeed with retry fail permanently
- **Missing patterns**: No standardized retry configuration across different handler types

### Files/Packages to Review in Next Session
1. **Handler Infrastructure**:
   - `example/shared/shell/command_handler_observability.go` (add retry status tracking)
   - `example/features/*/command_handler.go` (6 command handlers to update)
   - `example/features/*/query_handler.go` (3 query handlers to update)

2. **Error Classification** (determine retryable vs non-retryable):
   - `eventstore/postgresengine/postgres.go` (context errors, DB errors, concurrency conflicts)
   - `eventstore/errors.go` (ErrConcurrencyConflict, ErrInvalidPayloadJSON patterns)

3. **Configuration**:
   - `example/shared/shell/config/` (add retry configuration options)
   - Consider environment variables: `MAX_RETRIES`, `RETRY_BASE_DELAY`, `RETRY_MAX_DELAY`

### Next Session Implementation Plan
1. **Design Retry Configuration**: Create RetryConfig struct with max attempts, base delay, max delay, backoff multiplier
2. **Error Classification**: Implement `IsRetryableError()` helper to distinguish permanent vs transient failures
3. **Retry Logic Pattern**: Create reusable retry wrapper that works with existing handler patterns
4. **Observability Integration**: Add retry attempt counts, failure reasons, backoff delays to metrics
5. **Handler Updates**: Integrate retry logic into all command and query handlers
6. **Configuration Management**: Add retry settings to shell/config with sensible defaults

### Implementation Approach
- **Retryable Errors**: Context timeouts, temporary DB connection issues, some concurrency conflicts
- **Non-Retryable Errors**: Business rule violations, invalid payloads, permanent authentication issues
- **Backoff Strategy**: Exponential with jitter to avoid thundering herd effects
- **Observability**: Track retry attempts, success after retry, permanent failures
- **Configuration**: Environment-driven with production-safe defaults

### Success Criteria
- Configurable retry logic across all handlers
- Improved resilience under temporary infrastructure issues
- Proper error classification (retryable vs permanent)
- Complete observability for retry patterns
- Zero breaking changes to existing handler APIs


---

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

## Re-order Panels in Grafana Dashboard  
- **Created**: 2025-08-06
- **Priority**: Low - UX improvement for better dashboard usability
- **Objective**: Reorganize the 16-panel Grafana dashboard for better logical flow and visual hierarchy

### Current Dashboard Analysis
- **Current layout**: 16 panels in chronological order of implementation
- **User experience**: Related panels scattered across dashboard, non-intuitive flow
- **Visual hierarchy**: No clear grouping of related metrics (operations, errors, performance, context)

### Files/Packages to Review in Next Session
1. **Dashboard Configuration**:
   - `testutil/observability/grafana/dashboards/eventstore-dashboard.json` (panel positioning)
   - Panel `gridPos` coordinates and sizing for logical grouping

2. **Current Panel Inventory** (for reorganization planning):
   - **Operations**: Successful Append/Query Ops/sec (panels 1,2)  
   - **Errors**: Append/Query Errors/sec (panels 3,4)
   - **Performance**: Average durations (panels 7,8), SQL ops/sec (panels 5,6)
   - **Business Logic**: Command operations (panels 9,10), concurrency conflicts (panel 11), idempotent (panel 12)
   - **Context Errors**: Canceled operations (panels 13,14), timeout operations (panels 15,16)

### Next Session Implementation Plan
1. **Design Logical Groupings**: Group related panels into visual sections
2. **Proposed Layout**:
   - **Top Row**: Primary operations (successful append/query ops/sec)
   - **Second Row**: Performance metrics (average durations, P95/P99 if added)
   - **Third Row**: Error handling (errors/sec, concurrency conflicts)
   - **Fourth Row**: Context management (canceled, timeout operations)
   - **Bottom Row**: Business logic (command operations, idempotent)
3. **Update Panel Coordinates**: Modify `gridPos` for each panel
4. **Consider Panel Sizing**: Optimize panel widths for related metrics
5. **Visual Consistency**: Ensure consistent color schemes within groups

### Proposed Grouping Strategy
- **Core Operations Group**: Focus on primary EventStore operations and performance
- **Error Handling Group**: All error scenarios including business rules and infrastructure
- **Context Management Group**: Cancellation and timeout patterns
- **Business Logic Group**: Domain-specific operations and idempotency

### Success Criteria
- Logical flow from most important to least important metrics
- Related panels grouped visually 
- Improved dashboard usability for operations monitoring
- Maintained functionality of all 16 panels
- Clean visual layout with consistent spacing

---

## Query-Only Read Replica Usage for Pure Query Handlers
- **Created**: 2025-08-07
- **Priority**: Medium - Performance optimization for read-heavy workloads
- **Objective**: Implement dedicated read-only database connections for pure query handlers (non-command operations) to reduce load on primary database

### Current Architecture Analysis
- **Current Implementation**: Read/write splitting routes EventStore Query() operations to replica, but command handlers still use the same EventStore instance
- **Mixed Usage**: Command handlers use EventStore.Query() for business decisions (part of command processing) and EventStore.Append() for writes
- **Performance Opportunity**: Pure query handlers like `BooksInCirculation` could use dedicated read-only connections to further reduce primary load

### Future Implementation Approach
- **Query-Only Adapters**: Create specialized read-only database adapters that explicitly prevent write operations
- **Separate EventStore Instances**: Query handlers could use dedicated read-only EventStore instances backed by query-only adapters
- **Clear Architectural Separation**: Make read-only usage explicit rather than implicit routing within existing adapters
- **Replica Optimization**: Read replicas can be tuned specifically for query workloads, especially for handlers returning large datasets

### Files/Packages to Consider
1. **Query-Only Adapter Design**:
   - `eventstore/postgresengine/internal/adapters/readonly_pgx_adapter.go` - Read-only PGX adapter with Exec() disabled
   - `eventstore/postgresengine/internal/adapters/readonly_sql_adapter.go` - Read-only SQL adapter with Exec() disabled
   - `eventstore/postgresengine/internal/adapters/readonly_sqlx_adapter.go` - Read-only SQLX adapter with Exec() disabled

2. **Pure Query Handlers** (candidates for read-only EventStore):
   - `example/features/booksincirculation/` - Returns large datasets, perfect candidate for replica optimization
   - `example/features/bookslentout/` - Cross-entity queries that could benefit from replica tuning
   - `example/features/bookslentbyreader/` - Reader-specific queries that don't need write access

3. **EventStore Constructor Patterns**:
   - `eventstore/postgresengine/postgres.go` - Add `NewReadOnlyEventStoreFromPGXPool()` constructors
   - Query-only EventStore instances with compile-time prevention of Append() operations

### Benefits of Query-Only Approach
- **Explicit Read-Only Semantics**: Impossible to accidentally perform writes through query-only adapters
- **Primary Database Protection**: Pure query workloads completely offloaded from primary database
- **Replica Tuning**: Read replicas can be optimized specifically for large result set queries (different buffer settings, query cache, etc.)
- **Architectural Clarity**: Clear distinction between command processing (read/write) and pure query operations (read-only)
- **Performance Scaling**: Query handlers with large datasets (like `BooksInCirculation`) get dedicated resources

### Implementation Strategy
- **Fail-Fast Design**: Query-only adapters should panic or return errors if Exec() is called
- **Constructor Safety**: Read-only EventStore constructors prevent Append() method availability
- **Configuration Flexibility**: Allow query handlers to choose between shared EventStore (current) or dedicated read-only instances
- **Backward Compatibility**: Existing command handlers continue using current read/write splitting approach

### Success Criteria
- Query-only adapters explicitly prevent write operations at compile/runtime
- Pure query handlers can use dedicated read-only EventStore instances
- Primary database load reduced for query-intensive operations
- Clear architectural separation between command processing and pure queries
- Replica databases optimized specifically for query workloads

---

## Create Realistic Load Testing Scenarios  
- **Created**: 2025-08-04
- **Priority**: Medium - Depends on improved query handlers above
- **Objective**: Design sophisticated, realistic load generation scenarios that reflect real-world EventStore usage patterns

### Current Problem Analysis
- **Current load generator**: 50% scenarios result in idempotency (no events generated)
- **Business impact**: Unrealistic for load testing - real applications don't have 50% no-op scenarios
- **Testing limitation**: Can't properly test high-throughput EventStore performance
- **Scenario weights**: Current "20,80" (circulation, lending) creates too much idempotency

### Files/Packages to Review in Next Session
1. **Load Generator Core**:
   - `example/demo/cmd/load-generator/load_generator.go` (scenario selection logic)
   - `example/demo/cmd/load-generator/main.go` (configuration, scenario weights)

2. **Current Scenarios** (understand business logic causing idempotency):
   - `example/features/addbookcopy/` (circulation scenarios)
   - `example/features/removebookcopy/` (circulation scenarios)  
   - `example/features/lendbookcopytoreader/` (lending scenarios)
   - `example/features/returnbookcopyfromreader/` (lending scenarios)

3. **Business Logic** (why 50% scenarios generate no events):
   - `example/features/*/decide.go` (core business decision logic)
   - `example/shared/core/` (domain events and business rules)

4. **Command Handlers** (understand Query->Decide->Append pattern):
   - `example/features/*/command_handler.go` (where idempotency logic happens)

### Next Session Implementation Plan
1. **Analyze Current Scenarios**: Understand why 50% result in idempotency
2. **Design New Scenarios**: Create realistic business scenarios with <10% idempotency
3. **Implement Scenario Variants**: 
   - Real-world library usage patterns
   - State-dependent scenario selection
   - Time-based scenario distribution
4. **Add Scenario State Management**: Track library state to make informed scenario choices
5. **Test New Load Patterns**: Validate realistic append/query ratios (80-90% append success rate)

### Potential Solutions
- **State-aware scenarios**: Track which books are available vs lent out
- **Sequential scenarios**: Create realistic user journeys (register→lend→return→lend)
- **Weighted realistic distribution**: 70% lending operations, 25% circulation, 5% reader management
- **Smart scenario selection**: Avoid scenarios that would obviously fail business rules
- **Time-based patterns**: Simulate daily usage patterns (morning rush, evening returns)

### Success Criteria
- Append success rate: 80-90% (down from current 50%)
- Realistic load testing: Sustained high throughput without artificial idempotency
- Business scenario realism: Patterns that reflect actual library management usage
- Performance validation: Proper stress testing of EventStore under realistic conditions