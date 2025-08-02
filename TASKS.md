# TASKS.md

This file tracks larger plans, initiatives, and completed work for the Dynamic Event Streams EventStore project across multiple development sessions.

## 🚧 Current Plans (Ready to Implement)

### Comprehensive Testing Strategy for OpenTelemetry Adapters Package
- **Priority**: High
- **Description**: The current `trace_correlation_test.go` is insufficient for production-ready OpenTelemetry adapters
- **Problem Analysis**: 
  - No actual assertion of trace correlation (only logs output without verification)
  - Missing full OpenTelemetry setup (minimal tracer without proper exporters)
  - No metrics or tracing collector tests (only tests contextual logging)
  - No integration tests with actual EventStore operations
  - No error scenario or edge case testing
- **Implementation Plan**:
  - **New Test Files**: `metrics_collector_test.go`, `tracing_collector_test.go`, `contextual_logger_comprehensive_test.go` (replacement), `integration_test.go`, `otel_logger_test.go`
  - **Test Patterns**: Follow existing `testutil/` infrastructure with fluent assertions (`.WithOperation().WithStatus().Assert()`)
  - **Coverage Areas**: Interface compliance, OpenTelemetry instrument creation, attribute mapping, error handling, concurrency, EventStore integration
  - **Quality Standard**: Match the comprehensive testing approach used in main EventStore tests

### Documentation Review and Consistency Audit  
- **Priority**: Medium
- **Description**: Systematic review of all documentation for consistency and accuracy after recent architectural changes
- **Scope**: 
  - **Core Docs**: `README.md`, `docs/*.md` (getting-started, core-concepts, usage-examples, api-reference, development, performance)
  - **OpenTelemetry Docs**: `eventstore/oteladapters/README.md`, `eventstore/oteladapters/docs/*.md`
- **Focus Areas**:
  - Verify no old `/adapters/oteladapters` path references remain
  - Ensure OpenTelemetry integration examples are consistent across all documentation
  - Update feature lists to reflect new capabilities (contextual logging, trace correlation)
  - Review code examples for correct import paths and function signatures
  - Add comprehensive testing guidance to development documentation

---

## 🔄 In Progress

*(Currently empty)*

---

## ✅ Completed

### OpenTelemetry Ready-to-Use Adapters Package (Engine-Agnostic)
- **Completed**: 2025-08-02 (Updated package structure: 2025-08-02)
- **Description**: Engine-agnostic OpenTelemetry adapters providing plug-and-play integration for users with existing OpenTelemetry setups
- **Architecture Decision**: Moved observability interfaces from `postgresengine/options.go` to `eventstore/observability.go` for engine-agnostic design
- **Package Location**: `eventstore/oteladapters/` (not postgres-specific) - reusable by any future database engine
- **Features**:
  - **SlogBridgeLogger**: Uses official OpenTelemetry slog bridge for automatic trace correlation with zero config
  - **OTelLogger**: Direct OpenTelemetry logging API adapter for advanced control over log records
  - **MetricsCollector**: Maps EventStore metrics to OpenTelemetry instruments (histograms, counters, gauges)
  - **TracingCollector**: Creates OpenTelemetry spans with proper context propagation and status mapping
  - **Separate Go Module**: Independent dependencies to avoid forcing OpenTelemetry on core library users
  - **Complete Examples**: Full setup and slog-specific integration examples with production patterns
  - **Comprehensive Documentation**: Usage guide with architecture explanations and best practices
- **Future Benefit**: Any new database engine (MongoDB, DynamoDB) can reuse these same adapters without duplication

### OpenTelemetry-Compatible Contextual Logging (Dependency-Free)
- **Completed**: 2025-08-02
- **Description**: Complete observability triad with context-aware logging following the same dependency-free pattern as metrics and tracing
- **Features**: 
  - ContextualLogger interface using only standard library types for maximum flexibility
  - Automatic trace correlation when tracing is enabled - logs include span context automatically
  - Complete instrumentation of Query and Append operations with contextual logging
  - Dual logging support - traditional Logger and ContextualLogger work simultaneously
  - TestContextualLogger with comprehensive testing infrastructure
  - Zero dependencies - users integrate with any logging backend (OpenTelemetry, structured loggers)
  - Example implementation showing OpenTelemetry integration patterns
  - Backward compatible - existing Logger interface unchanged

### Observability Code Readability Improvements
- **Completed**: 2025-08-02  
- **Description**: Significant code readability improvements using observer patterns to reduce observability noise
- **Features**:
  - Metrics observer pattern - simplified metrics recording with queryMetricsObserver and appendMetricsObserver
  - Tracing observer pattern - encapsulated span lifecycle management
  - Reduced postgres.go complexity - observability calls simplified from 6+ calls to 2-3 simple observer calls
  - Maintained identical functionality - zero breaking changes to external API
  - Consistent patterns - all observability follows same observer pattern for maintainability

### Distributed Tracing Support (Dependency-Free)
- **Completed**: 2025-08-01
- **Description**: Comprehensive distributed tracing following the same dependency-free pattern as existing metrics
- **Features**: 
  - TracingCollector and SpanContext interfaces using only standard library types
  - Complete instrumentation of Query and Append operations with span context propagation
  - Full error tracking with detailed error classification and span status codes
  - TestTracingCollector with fluent assertion interface for comprehensive testing
  - Zero dependencies - users integrate with any tracing backend (OpenTelemetry, Jaeger, Zipkin)
  - Production-ready with proper concurrency safety and optional tracing via functional options

### OpenTelemetry-Compatible Metrics Collection
- **Completed**: 2025-01-31
- **Description**: Comprehensive metrics instrumentation with duration, counters, and error tracking
- **Features**: 
  - MetricsCollector interface
  - Automatic metrics collection for all operations
  - OpenTelemetry-compatible labels and conventions
  - Complete test suite with TestMetricsCollector
  - Full documentation coverage

---

## 💡 Future Ideas

*(Currently empty)*

---

*Last Updated: 2025-08-02*