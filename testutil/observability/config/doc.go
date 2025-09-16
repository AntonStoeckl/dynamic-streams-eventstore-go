// Package config provides observability configuration for EventStore testing.
//
// This package contains factory functions for setting up OpenTelemetry-compatible
// observability providers (metrics, tracing, and structured logging) needed
// for testing EventStore's observability features.
//
// The configurations create test-appropriate providers that can be used
// to verify that EventStore operations properly emit metrics, traces,
// and logs without requiring external observability infrastructure.
package config
