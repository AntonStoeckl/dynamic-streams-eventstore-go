// Package registerreader implements the Register Reader use case.
//
// This feature allows registering new readers in the library system.
// It follows the Command-Query-Decide-Append pattern with proper separation between
// infrastructure concerns (CommandHandler) and pure business logic (Decide function).
//
// The business logic ensures idempotency - attempting to register a reader that
// already exists will result in a no-op (no events generated).
//
// In a real application there should be a unit test for this business logic, unless coarse-grained
// feature tests are preferred. Complex business logic typically benefits from dedicated pure unit tests.
package registerreader
