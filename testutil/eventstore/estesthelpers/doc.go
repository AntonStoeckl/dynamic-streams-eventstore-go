// Package estesthelpers provides EventStore-agnostic test utilities and fixtures.
//
// This package contains test helpers that work with any EventStore implementation,
// focusing on domain event creation, filter building, and common test operations.
//
// Test ID Generation:
//
//	GivenUniqueID: generates UUID v7 for test entity IDs
//
// Filter Builders:
//
//	FilterAllEventTypesForOneBook: creates filter for all book events
//	FilterAllEventTypesForOneBookOrReader: creates filter for book and reader events
//
// Event Fixtures:
//
//	FixtureBookCopyAddedToCirculation: creates book addition test event
//	FixtureBookCopyRemovedFromCirculation: creates book removal test event
//	FixtureBookCopyLentToReader: creates book lending test event
//	FixtureBookCopyReturnedByReader: creates book return test event
//
// Event Conversion:
//
//	ToStorable: converts domain event to storable event with empty metadata
//	ToStorableWithMetadata: converts domain event with custom metadata
//
// Test Data Setup:
//
//	GivenBookCopyAddedToCirculationWasAppended: appends book addition event
//	GivenBookCopyRemovedFromCirculationWasAppended: appends book removal event
//	GivenBookCopyLentToReaderWasAppended: appends book lending event
//	GivenBookCopyReturnedByReaderWasAppended: appends book return event
//
// These utilities support the library management domain used throughout EventStore tests.
package estesthelpers
