package config

import (
	"context"
	"log"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// JaegerEndpoint returns the Jaeger OTLP gRPC endpoint for testing the observability stack.
func JaegerEndpoint() string {
	return "localhost:4319"
}

// OTELCollectorEndpoint returns the OpenTelemetry Collector gRPC endpoint for test observability stack.
func OTELCollectorEndpoint() string {
	return "localhost:4317"
}

// TestObservabilityProviders holds the OpenTelemetry providers for testing.
type TestObservabilityProviders struct {
	TracerProvider *trace.TracerProvider
	MeterProvider  *metric.MeterProvider
	Resource       *resource.Resource
}

// NewTestObservabilityConfig creates OpenTelemetry providers configured for the test observability stack.
// This function sets up real OpenTelemetry providers that send telemetry to the observability backends
// running in Docker containers (Prometheus, Jaeger, OTEL Collector).
func NewTestObservabilityConfig() (*TestObservabilityProviders, error) {
	ctx := context.Background()

	// Create a resource for identifying this service
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String("eventstore-test"),
			semconv.ServiceVersionKey.String("test"),
		),
	)
	if err != nil {
		log.Fatal("Failed to create resource: ", err)
	}

	// Set up trace provider with OTLP exporter pointing directly to Jaeger
	traceExporter, err := otlptracegrpc.New(
		ctx,
		otlptracegrpc.WithEndpoint(JaegerEndpoint()),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	tracerProvider := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter),
		trace.WithResource(res),
	)

	// Set up a metrics provider with OTLP exporter
	metricExporter, err := otlpmetricgrpc.New(
		ctx,
		otlpmetricgrpc.WithEndpoint(OTELCollectorEndpoint()),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(metricExporter,
			metric.WithInterval(5*time.Second))),
		metric.WithResource(res),
	)

	// Set global providers for OpenTelemetry
	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return &TestObservabilityProviders{
		TracerProvider: tracerProvider,
		MeterProvider:  meterProvider,
		Resource:       res,
	}, nil
}

// Shutdown gracefully shuts down the OpenTelemetry providers.
func (p *TestObservabilityProviders) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var err error
	if shutdownErr := p.TracerProvider.Shutdown(ctx); shutdownErr != nil {
		err = shutdownErr
	}

	if shutdownErr := p.MeterProvider.Shutdown(ctx); shutdownErr != nil {
		if err != nil {
			log.Printf("Multiple shutdown errors occurred. First: %v, Second: %v", err, shutdownErr)
		}
		err = shutdownErr
	}

	return err
}
