// Package readercontractcanceled implements the Reader Contract Canceled use case.
//
// This feature allows canceling a reader's contract in the library system.
// It follows the Command-Query-Decide-Append pattern with proper separation between
// infrastructure concerns (CommandHandler) and pure business logic (Decide function).
//
// The business logic ensures idempotency - attempting to cancel a contract for a reader
// that is not registered or already canceled will result in a no-op (no events generated).
//
// In a real application there should be a unit test for this business logic, unless coarse-grained
// feature tests are preferred. Complex business logic typically benefits from dedicated pure unit tests.
package readercontractcanceled
