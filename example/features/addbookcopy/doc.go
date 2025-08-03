// Package addbookcopy implements the Add Book Copy to Circulation use case.
//
// This feature allows adding new book copies to the library's circulation system.
// It follows the Command-Query-Decide-Append pattern with proper separation between
// infrastructure concerns (CommandHandler) and pure business logic (Decide function).
//
// The business logic ensures idempotency - attempting to add a book copy that
// already exists in circulation will result in a no-op (no events generated).
//
// In a real application there should be a unit test for this business logic, unless coarse-grained
// feature tests are preferred. Complex business logic typically benefits from dedicated pure unit tests.
package addbookcopy
