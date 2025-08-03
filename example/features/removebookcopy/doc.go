// Package removebookcopy implements the "Remove Book Copy from Circulation" use case
// following Vertical Feature Slice architecture.
//
// This package demonstrates proper separation of core domain logic from shell infrastructure
// concerns within a single cohesive feature slice:
//
// Core (decide.go): Pure Decide() function with no side effects
// Shell (command_handler.go, command.go): Infrastructure orchestration and DTOs
//
// This feature uses shared domain events from shared/core and infrastructure
// utilities from shared/shell for event conversion and storage operations.
//
// The CommandHandler encapsulates the complete Query() -> Decide() -> Append() cycle,
// while the pure Decide() function contains only business rules and is easily testable.
//
// In a real application there should be a unit test for this business logic, unless coarse-grained
// feature tests are preferred. Complex business logic typically benefits from dedicated pure unit tests.
//
// Note: In a real application, the BuildCommand() function should validate input
// parameters if necessary. Important domain concepts like BookID might be better
// encapsulated in value objects rather than primitive types for stronger type safety,
// immutability, and domain modeling.
package removebookcopy
