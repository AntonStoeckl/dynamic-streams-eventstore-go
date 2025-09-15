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
//		observable.WithCommandMetrics(metricsCollector),
//		observable.WithCommandTracing(tracingCollector),
//		observable.WithCommandContextualLogging(contextualLogger),
//	)
//
//	// 3. Use wrapped handler in application
//	err = observableHandler.Handle(ctx, command)
//
// # Query Handler Usage
//
// Query handlers follow the same pattern with type-safe generics:
//
//	// 1. Create clean query handler
//	coreHandler := booksincirculation.NewQueryHandler(eventStore)
//
//	// 2. Optionally wrap with snapshot optimization
//	snapshotHandler, err := snapshot.NewQueryWrapper(
//		coreHandler,
//		eventStore,
//		booksincirculation.Project,
//		func(_ booksincirculation.Query) eventstore.Filter {
//			return booksincirculation.BuildEventFilter()
//		},
//	)
//
//	// 3. Wrap with observability
//	observableHandler, err := observable.NewQueryWrapper(
//		snapshotHandler, // or coreHandler directly
//		observable.WithQueryMetrics[booksincirculation.Query, booksincirculation.BooksInCirculation](metricsCollector),
//		observable.WithQueryTracing[booksincirculation.Query, booksincirculation.BooksInCirculation](tracingCollector),
//	)
//
//	// 4. Use in application
//	result, err := observableHandler.Handle(ctx, booksincirculation.BuildQuery())
//
// # Composition Architecture
//
// The architecture supports flexible composition:
//
//	Core Handler → Snapshot Wrapper → Observable Wrapper
//	     ↓              ↓                    ↓
//	Pure business   Performance         Observability
//	    logic       optimization       instrumentation
//
// # Selective Observability
//
// You can choose which observability concerns to apply:
//
//	// Only metrics and basic logging for commands
//	wrapper, err := observable.NewCommandWrapper(
//		coreHandler,
//		observable.WithCommandMetrics(metricsCollector),
//		observable.WithCommandLogging(logger),
//	)
//
//	// Only tracing for queries
//	wrapper, err := observable.NewQueryWrapper(
//		queryHandler,
//		observable.WithQueryTracing[Q, R](tracingCollector),
//	)
//
// # Pure Business Logic Testing
//
// For unit tests focused on business logic, use handlers without observability:
//
//	// Pure command handler - no observability overhead
//	handler := lendbookcopytoreader.NewCommandHandler(eventStore)
//	err := handler.Handle(ctx, command)  // Direct business logic execution
//
//	// Pure query handler - no observability overhead
//	handler := booksincirculation.NewQueryHandler(eventStore)
//	result, err := handler.Handle(ctx, query)  // Direct business logic execution
//
// # Interface Compatibility
//
// All wrappers implement the same interfaces as their core handlers:
//   - CommandWrapper implements shell.CommandHandler[C]
//   - QueryWrapper implements shell.QueryHandler[Q, R]
//
// This enables complete interchangeability and composition flexibility.
//
// # Architecture Benefits
//
//   - Handlers contain ONLY business logic (Query → Unmarshal → Decide → Append for commands)
//   - All observability is optional and composable
//   - Clear separation between business logic and infrastructure concerns
//   - Easy to test business logic without observability complexity
//   - Consistent composition pattern across command and query handlers
//   - External wrapping makes observability composition explicit and transparent
//
// # Code Reduction
//
// Using this pattern dramatically reduces handler code:
//   - Command handlers: ~60% reduction (~253 lines → ~86 lines of pure business logic)
//   - Query handlers: ~70% reduction (removed all observability, options, Expose* methods)
//   - Business workflow is immediately visible and understandable
package observable
