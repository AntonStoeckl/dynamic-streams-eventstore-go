// Package lendbookcopytoreader implements the Lend Book Copy to Reader use case.
//
// This feature allows lending available book copies to registered readers.
// It follows the Command-Query-Decide-Append pattern with proper separation between
// infrastructure concerns (CommandHandler) and pure business logic (Decide function).
//
// The business logic enforces multiple constraints: books must be in circulation,
// not currently lent to anyone, and readers cannot exceed their lending limit (max 10 books).
// Failed operations generate appropriate error events using the SomethingHasHappened pattern.
//
// In a real application there should be a unit test for this business logic, unless coarse-grained
// feature tests are preferred. Complex business logic typically benefits from dedicated pure unit tests.
package lendbookcopytoreader
