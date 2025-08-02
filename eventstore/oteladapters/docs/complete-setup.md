# Complete OpenTelemetry Setup

This example demonstrates a complete OpenTelemetry setup with all three observability adapters: logging, metrics, and tracing.

## Full Integration Example

```go
package main

import (
	"context"
	"log"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/oteladapters"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/trace"
)

func main() {
	ctx := context.Background()

	// 1. Setup OpenTelemetry Tracing
	traceExporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		log.Fatal("Failed to create trace exporter:", err)
	}

	tracerProvider := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter),
	)
	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// 2. Setup OpenTelemetry Metrics
	metricExporter, err := stdoutmetric.New()
	if err != nil {
		log.Fatal("Failed to create metric exporter:", err)
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(metricExporter)),
	)
	otel.SetMeterProvider(meterProvider)

	// 3. Setup OpenTelemetry Logging
	logExporter, err := stdoutlog.New()
	if err != nil {
		log.Fatal("Failed to create log exporter:", err)
	}

	loggerProvider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
	)
	global.SetLoggerProvider(loggerProvider)

	// 4. Create EventStore adapters
	tracer := otel.Tracer("eventstore")
	meter := otel.Meter("eventstore")

	// Create OpenTelemetry adapters
	tracingCollector := oteladapters.NewTracingCollector(tracer)
	metricsCollector := oteladapters.NewMetricsCollector(meter)
	
	// For logging, use the slog bridge with automatic trace correlation
	contextualLogger := oteladapters.NewSlogBridgeLogger("eventstore")

	// 5. Setup PostgreSQL connection
	pgxPool, err := pgxpool.New(ctx, "postgres://user:password@localhost/eventstore")
	if err != nil {
		log.Fatal("Failed to create pgx pool:", err)
	}
	defer pgxPool.Close()

	// 6. Create EventStore with all OpenTelemetry adapters
	eventStore, err := postgresengine.NewEventStoreFromPGXPool(
		pgxPool,
		postgresengine.WithTracing(tracingCollector),
		postgresengine.WithMetrics(metricsCollector),
		postgresengine.WithContextualLogger(contextualLogger),
		postgresengine.WithTableName("events"),
	)
	if err != nil {
		log.Fatal("Failed to create event store:", err)
	}

	// 7. Use the EventStore - all operations will be traced, logged, and measured
	_ = eventStore // Use your eventStore here for actual operations
	
	log.Println("EventStore created with complete OpenTelemetry observability!")

	// 8. Cleanup
	defer func() {
		if err := tracerProvider.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
		if err := meterProvider.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down meter provider: %v", err)
		}
		if err := loggerProvider.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down logger provider: %v", err)
		}
	}()
}
```

## Production Setup

In production, you would typically use:

- **OTLP exporters** instead of stdout exporters
- **Environment variables** for configuration
- **Proper error handling** and logging
- **Resource detection** and attribution

### Example Environment Variables

```bash
# OpenTelemetry Configuration
OTEL_EXPORTER_OTLP_ENDPOINT=https://your-otel-collector:4317
OTEL_SERVICE_NAME=eventstore-service
OTEL_SERVICE_VERSION=1.0.0
OTEL_ENVIRONMENT=production

# Additional optional configuration
OTEL_RESOURCE_ATTRIBUTES=service.instance.id=instance-1,deployment.environment=prod
OTEL_EXPORTER_OTLP_HEADERS=api-key=your-api-key
```

### Production Code Pattern

```go
// Use environment-driven configuration
provider := trace.NewTracerProvider(
	trace.WithResource(resource.Default()),
	trace.WithBatcher(otlptrace.New(ctx, otlptrace.WithInsecure())),
)

// The EventStore setup remains the same
eventStore, err := postgresengine.NewEventStoreFromPGXPool(
	pgxPool,
	postgresengine.WithTracing(oteladapters.NewTracingCollector(tracer)),
	postgresengine.WithMetrics(oteladapters.NewMetricsCollector(meter)),
	postgresengine.WithContextualLogger(oteladapters.NewSlogBridgeLogger("eventstore")),
)
```

## What You Get

With this setup, every EventStore operation will automatically:

1. **Create spans** with timing and error information
2. **Record metrics** for durations, counts, and errors
3. **Log with trace correlation** including trace/span IDs
4. **Propagate context** across service boundaries