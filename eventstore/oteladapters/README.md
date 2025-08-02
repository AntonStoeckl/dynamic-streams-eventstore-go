# OpenTelemetry Adapters for Dynamic Event Streams EventStore

This package provides ready-to-use [OpenTelemetry](https://opentelemetry.io/) adapters for the [dynamic-streams-eventstore-go](https://github.com/AntonStoeckl/dynamic-streams-eventstore-go) library's observability interfaces.

## 🎯 Purpose

The EventStore library uses dependency-free observability interfaces (`Logger`, `ContextualLogger`, `MetricsCollector`, `TracingCollector`) to avoid forcing specific observability dependencies on users. This package bridges those interfaces to OpenTelemetry, providing:

- **Zero-config integration** for OpenTelemetry users
- **Automatic trace correlation** in logs
- **Production-ready implementations** using OpenTelemetry best practices
- **Engine-agnostic design** - works with any EventStore engine (PostgreSQL, future MongoDB, etc.)

## 🚀 Quick Start

### Install the Package

```bash
go get github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/oteladapters
```

### Basic Usage

```go
import (
    "github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/adapters/otel"
    "github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine"
    otelglobal "go.opentelemetry.io/otel"
)

// Setup OpenTelemetry (tracer, meter, logger)
tracer := otelglobal.Tracer("eventstore")
meter := otelglobal.Meter("eventstore")

// Create OpenTelemetry adapters
tracingCollector := oteladapters.NewTracingCollector(tracer)
metricsCollector := oteladapters.NewMetricsCollector(meter)

// For logging, use the slog bridge with automatic trace correlation
contextualLogger := oteladapters.NewSlogBridgeLogger("eventstore")

// Create EventStore with OpenTelemetry observability
eventStore, err := postgresengine.NewEventStoreFromPGXPool(
    pgxPool,
    postgresengine.WithTracing(tracingCollector),
    postgresengine.WithMetrics(metricsCollector),
    postgresengine.WithContextualLogger(contextualLogger),
)
```

## 📦 Available Adapters

### 1. Contextual Logger Adapters

#### SlogBridgeLogger (Recommended)
Uses the official OpenTelemetry slog bridge for automatic trace correlation:

```go
// Option 1: Pure OpenTelemetry with automatic trace correlation
logger := oteladapters.NewSlogBridgeLogger("eventstore")

// Option 2: Use your existing slog.Handler (no trace correlation)
slogHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
logger := oteladapters.NewSlogBridgeLoggerWithHandler(slogHandler)
```

**Benefits:**
- ✅ **Automatic trace/span ID injection** when using `NewSlogBridgeLogger()`
- ✅ **Zero configuration** - uses global OpenTelemetry LoggerProvider
- ✅ **Handler compatibility** - `NewSlogBridgeLoggerWithHandler()` for existing setups
- ✅ **Production-ready** with minimal setup

**Trace Correlation Example:**
```json
// Without trace context:
{"level":"INFO","msg":"Query executed","duration":"150ms"}

// With active trace context:
{"level":"INFO","msg":"Query executed","duration":"150ms","trace_id":"abc123","span_id":"def456"}
```

#### OTelLogger (Advanced)
Direct OpenTelemetry logging API integration:

```go
otelLogger := // your otel/log.Logger
logger := oteladapters.NewOTelLogger(otelLogger)
```

**Use when:**
- You need direct control over OpenTelemetry log records
- You're not using Go's standard `log/slog` package
- You have custom OpenTelemetry logging requirements

### 2. Metrics Collector

Maps EventStore metrics to OpenTelemetry instruments:

```go
meter := otel.Meter("eventstore")
collector := oteladapters.NewMetricsCollector(meter)
```

**Instrument Mapping:**
- `RecordDuration(...)` → **Histogram** (for operation durations)
- `IncrementCounter(...)` → **Counter** (for operation counts, errors)
- `RecordValue(...)` → **Gauge** (for current values, concurrent operations)

### 3. Tracing Collector

Creates OpenTelemetry spans for EventStore operations:

```go
tracer := otel.Tracer("eventstore")
collector := oteladapters.NewTracingCollector(tracer)
```

**Features:**
- Automatic span creation for Query/Append operations
- Context propagation across operations
- Error status mapping and attribute injection
- Proper span lifecycle management

## 📋 Complete Examples

📖 **[Complete OpenTelemetry Setup](docs/complete-setup.md)**
- Full OpenTelemetry tracing, metrics, and logging setup
- EventStore integration with all adapters
- Production configuration patterns
- Proper cleanup and error handling

📖 **[Slog Integration Examples](docs/slog-integration.md)**
- Slog with OpenTelemetry trace correlation
- Slog-only integration (without full OpenTelemetry)
- Custom slog handler integration
- Best practices and common patterns

## 🏗️ Architecture

### Engine-Agnostic Design

These adapters are **not** PostgreSQL-specific. They implement the generic `eventstore` observability interfaces and can be used with any database engine:

```
eventstore/
├── observability.go              # Generic interfaces
├── postgresengine/               # PostgreSQL implementation
│   └── uses eventstore interfaces
├── future-mongodb-engine/        # Future MongoDB implementation  
│   └── uses same eventstore interfaces
└── oteladapters/                # Engine-agnostic OpenTelemetry adapters
    └── implements eventstore interfaces
```

### Dependencies

This package is a **separate Go module** to avoid adding OpenTelemetry dependencies to the core EventStore library:

```go
// Core library - no OpenTelemetry dependencies
go get github.com/AntonStoeckl/dynamic-streams-eventstore-go

// OpenTelemetry adapters - optional
go get github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/oteladapters
```

## 🔧 Configuration

### Production Setup

For production environments, consider:

```go
// Use OTLP exporters instead of stdout
traceExporter := otlptrace.New(...)
metricExporter := otlpmetric.New(...)
logExporter := otlplog.New(...)

// Configure with environment variables
// OTEL_EXPORTER_OTLP_ENDPOINT=https://your-collector:4317
// OTEL_SERVICE_NAME=your-service
// OTEL_SERVICE_VERSION=1.0.0
// OTEL_ENVIRONMENT=production
```

### Sampling and Performance

```go
// Configure trace sampling for high-throughput scenarios
tracerProvider := trace.NewTracerProvider(
    trace.WithSampler(trace.TraceIDRatioBased(0.1)), // 10% sampling
    trace.WithBatcher(traceExporter),
)
```

## 🧪 Testing

All adapters implement the EventStore observability interfaces and can be tested using the same test utilities:

```go
// Use EventStore test utilities for validation
import "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper"

// Adapters work with existing test infrastructure
testCollector := helper.NewTestMetricsCollector()
// ... test your integration
```

## 📚 Related Documentation

- [EventStore Main Documentation](../../README.md)
- [OpenTelemetry Go Documentation](https://opentelemetry.io/docs/languages/go/)
- [OpenTelemetry Slog Bridge](https://pkg.go.dev/go.opentelemetry.io/contrib/bridges/otelslog)

## 🤝 Contributing

This package follows the same contribution guidelines as the main EventStore library. When adding new adapters or features:

1. Maintain engine-agnostic design
2. Follow OpenTelemetry best practices  
3. Include comprehensive examples
4. Add appropriate tests
5. Update documentation

## 📄 License

Same license as the main EventStore library (GPL v3).