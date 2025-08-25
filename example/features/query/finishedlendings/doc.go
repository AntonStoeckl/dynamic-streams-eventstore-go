// Package finishedlendings provides query functionality for retrieving all lending cycles that have been completed.
//
// This query handler tracks books that have been lent out and subsequently returned through the
// BookCopyReturnedByReader events. It provides a complete view of all finished lending transactions,
// which is essential for cleanup operations that need to identify and delete events related to
// lending cycles that are logically "finished" and no longer active in the system.
//
// The query only processes return events and tracks the completion of lending cycles,
// focusing specifically on the transactions that have been fully completed (lent and returned).
package finishedlendings
