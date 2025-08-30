// Package observable provides wrapper components for instrumenting command and query handlers
// with comprehensive observability (metrics, tracing, logging) while keeping business logic pure.
//
// This package follows the same composition pattern as the snapshot wrapper to maintain
// architectural consistency and provide external, explicit wrapping of handlers.
//
// # Core Principle: External Wrapping
//
// The observable wrappers are applied externally at bootstrap/wiring time, not hidden
// inside factory functions. This makes the observability composition explicit and transparent.
//
// # Command Handler Usage
//
// Basic pattern for wrapping a command handler with observability:
//
//	// 1. Create pure business logic handler
//	coreHandler := lendbookcopytoreader.NewCommandHandler(eventStore)
//
//	// 2. Wrap with observability (external, explicit)
//	observableHandler, err := observable.NewCommandWrapper(
//		coreHandler,
//		observable.WithMetrics(metricsCollector),
//		observable.WithTracing(tracingCollector),
//		observable.WithContextualLogging(contextualLogger),
//	)
//
//	// 3. Use wrapped handler in application
//	err = observableHandler.Handle(ctx, command)
//
// # Selective Observability
//
// You can choose which observability concerns to apply:
//
//	// Only metrics and basic logging
//	wrapper, err := observable.NewCommandWrapper(
//		coreHandler,
//		observable.WithMetrics(metricsCollector),
//		observable.WithLogging(logger),
//	)
//
//	// Only tracing
//	wrapper, err := observable.NewCommandWrapper(
//		coreHandler,
//		observable.WithTracing(tracingCollector),
//	)
//
// # Pure Business Logic Testing
//
// For unit tests focused on business logic, use handlers without observability:
//
//	// Pure handler - no observability overhead
//	handler := lendbookcopytoreader.NewCommandHandler(eventStore)
//	err := handler.Handle(ctx, command)  // Direct business logic execution
//
// # Architecture Benefits
//
//   - Command handlers contain ONLY business logic (Query → Unmarshal → Decide → Append)
//   - All observability is optional and composable
//   - Clear separation between business logic and infrastructure concerns
//   - Easy to test business logic without observability complexity
//   - Consistent with existing snapshot wrapper pattern
//   - External wrapping makes observability composition explicit and transparent
//
// # Code Reduction
//
// Using this pattern reduces command handler code by ~60%:
//   - Before: ~253 lines with mixed concerns
//   - After: ~86 lines of pure business logic + shared observable wrapper
//   - Business workflow is immediately visible and understandable
package observable
