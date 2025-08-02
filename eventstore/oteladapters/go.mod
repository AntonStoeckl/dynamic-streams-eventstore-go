module github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/oteladapters

go 1.24

require (
	github.com/AntonStoeckl/dynamic-streams-eventstore-go v1.2.0-beta
	go.opentelemetry.io/contrib/bridges/otelslog v0.12.0
	go.opentelemetry.io/otel v1.37.0
	go.opentelemetry.io/otel/log v0.13.0
	go.opentelemetry.io/otel/metric v1.37.0
	go.opentelemetry.io/otel/sdk v1.37.0
	go.opentelemetry.io/otel/trace v1.37.0
)

require (
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	golang.org/x/sys v0.34.0 // indirect
)

// Local development - use local eventstore package
replace github.com/AntonStoeckl/dynamic-streams-eventstore-go => ../..
