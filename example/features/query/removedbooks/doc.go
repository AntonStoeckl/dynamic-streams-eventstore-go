// Package removedbooks provides query functionality for retrieving all books that have been removed from circulation.
//
// This query handler tracks books that have been removed from the library's circulation through the
// BookCopyRemovedFromCirculation events. It provides a complete view of all books that are no longer
// available for lending, which is essential for cleanup operations that need to identify and delete
// events related to books that are logically "gone" from the system.
//
// The query only processes removal events and ignores books that are currently in circulation,
// focusing specifically on the books that have been permanently removed from the library's inventory.
package removedbooks
