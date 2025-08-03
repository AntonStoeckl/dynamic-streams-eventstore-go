// Package returnbookcopyfromreader implements the Return Book Copy from Reader use case.
//
// This feature allows readers to return previously lent book copies back to circulation.
// It follows the Command-Query-Decide-Append pattern with proper separation between
// infrastructure concerns (CommandHandler) and pure business logic (Decide function).
//
// The business logic enforces multiple constraints with fine-grained error handling:
// books must be in circulation, must be lent to the specific reader requesting the return.
// Failed operations generate specific error events using the SomethingHasHappened pattern.
//
// In a real application there should be a unit test for this business logic, unless coarse-grained
// feature tests are preferred. Complex business logic typically benefits from dedicated pure unit tests.
package returnbookcopyfromreader