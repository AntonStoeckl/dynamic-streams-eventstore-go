# Slog Integration Examples

This document shows different ways to integrate the EventStore with Go's standard `log/slog` package and OpenTelemetry for automatic trace correlation.

## Full Slog + OpenTelemetry Integration

This example shows how to get **automatic trace correlation** in your logs.

```go
package main

import (
	"context"
	"log"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/oteladapters"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/trace"
)

func main() {
	ctx := context.Background()

	// 1. Setup basic OpenTelemetry tracing (for trace correlation)
	traceExporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		log.Fatal("Failed to create trace exporter:", err)
	}

	tracerProvider := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter),
	)
	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// 2. Create OpenTelemetry slog bridge logger with automatic trace correlation
	// This automatically adds trace and span IDs to log messages when tracing is active
	contextualLogger := oteladapters.NewSlogBridgeLogger("eventstore")

	// 3. Optionally, create tracing collector for EventStore spans
	tracer := otel.Tracer("eventstore")
	tracingCollector := oteladapters.NewTracingCollector(tracer)

	// 4. Setup PostgreSQL connection
	pgxPool, err := pgxpool.New(ctx, "postgres://user:password@localhost/eventstore")
	if err != nil {
		log.Fatal("Failed to create pgx pool:", err)
	}
	defer pgxPool.Close()

	// 5. Create EventStore with slog integration
	eventStore, err := postgresengine.NewEventStoreFromPGXPool(
		pgxPool,
		postgresengine.WithContextualLogger(contextualLogger),
		postgresengine.WithTracing(tracingCollector), // Optional: adds spans
	)
	if err != nil {
		log.Fatal("Failed to create event store:", err)
	}

	// 6. Use the EventStore - all log messages will include trace context
	_ = eventStore // Use your eventStore here

	// Example: Log with context that will include trace information
	contextualLogger.InfoContext(ctx, "EventStore initialized with slog and OpenTelemetry",
		"database", "postgresql",
		"table", "events",
	)

	log.Println("EventStore created with slog and OpenTelemetry trace correlation!")

	// 7. Cleanup
	defer func() {
		if err := tracerProvider.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}()
}
```

### Expected Log Output

**Without trace context:**
```json
{"level":"INFO","msg":"EventStore initialized","database":"postgresql","table":"events"}
```

**With active trace context:**
```json
{
  "level":"INFO",
  "msg":"EventStore initialized", 
  "database":"postgresql",
  "table":"events",
  "trace_id":"abc123",
  "span_id":"def456"
}
```

## Slog-Only Integration (Without OpenTelemetry)

For simpler scenarios where you just want the `ContextualLogger` interface:

```go
package main

import (
	"context"
	"log"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/oteladapters"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	ctx := context.Background()

	// 1. Create the slog bridge logger with minimal setup
	// Even without full OpenTelemetry setup, this provides the ContextualLogger interface
	contextualLogger := oteladapters.NewSlogBridgeLogger("eventstore")

	// 2. Setup PostgreSQL connection
	pgxPool, err := pgxpool.New(ctx, "postgres://user:password@localhost/eventstore")
	if err != nil {
		log.Fatal("Failed to create pgx pool:", err)
	}
	defer pgxPool.Close()

	// 3. Create EventStore with just contextual logging
	eventStore, err := postgresengine.NewEventStoreFromPGXPool(
		pgxPool,
		postgresengine.WithContextualLogger(contextualLogger),
	)
	if err != nil {
		log.Fatal("Failed to create event store:", err)
	}

	// 4. Use the EventStore - logs will use OpenTelemetry's logger
	_ = eventStore // Use your eventStore here

	log.Println("EventStore created with slog-only integration!")
}
```

## Custom Slog Handler Integration

If you have an existing slog setup and want to preserve it:

```go
package main

import (
	"log/slog"
	"os"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/oteladapters"
)

func main() {
	// 1. Create your custom slog handler
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
		AddSource: true,
	})

	// You could also create a multi-handler that writes to multiple destinations:
	// multiHandler := NewMultiHandler(jsonHandler, fileHandler, networkHandler)

	// 2. Create the slog bridge logger with your custom handler
	// Note: This does NOT add OpenTelemetry trace correlation
	contextualLogger := oteladapters.NewSlogBridgeLoggerWithHandler(jsonHandler)

	// 3. Use as in previous examples...
	_ = contextualLogger // Use your contextual logger here

	log.Println("Custom slog handler integration example!")
}
```

## Key Differences

| Function                                  | Trace Correlation | Use Case                                |
|-------------------------------------------|-------------------|-----------------------------------------|
| `NewSlogBridgeLogger("name")`             | ✅ **YES**         | Want automatic trace/span IDs in logs   |
| `NewSlogBridgeLoggerWithHandler(handler)` | ❌ **NO**          | Want to use existing slog.Handler as-is |

## Best Practices

1. **Use `NewSlogBridgeLogger()`** if you want trace correlation
2. **Set up OpenTelemetry tracing** before creating the logger for correlation to work
3. **Use structured logging** with key-value pairs for better observability
4. **Include context** in all log calls to enable trace correlation

## Common Patterns

### With Request Context
```go
func handleRequest(ctx context.Context, logger eventstore.ContextualLogger) {
	// This will automatically include trace/span IDs if tracing is active
	logger.InfoContext(ctx, "Processing request", 
		"user_id", "123",
		"operation", "query_events",
	)
}
```

### Error Logging with Context
```go
func processEvents(ctx context.Context, logger eventstore.ContextualLogger) error {
	if err := someOperation(); err != nil {
		// Error logs with trace correlation help with debugging
		logger.ErrorContext(ctx, "Failed to process events",
			"error", err.Error(),
			"retry_count", 3,
		)
		return err
	}
	return nil
}
```