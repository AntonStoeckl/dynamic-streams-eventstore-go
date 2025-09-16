package shared

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	jsoniter "github.com/json-iterator/go"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
)

// ErrMappingToEventMetadataFailed is returned when metadata conversion fails.
var ErrMappingToEventMetadataFailed = errors.New("mapping to event metadata failed")

// QueriesAndAppendsEvents abstracts EventStore operations for test helpers.
// This interface enables test utilities to work with any EventStore implementation
// (PostgreSQL, future backends) by providing the essential query and append operations
// needed for test setup and verification.
type QueriesAndAppendsEvents interface {
	Query(ctx context.Context, filter eventstore.Filter) (eventstore.StorableEvents, eventstore.MaxSequenceNumberUint, error)
	Append(ctx context.Context, filter eventstore.Filter, maxSeq eventstore.MaxSequenceNumberUint, events ...eventstore.StorableEvent) error
}

// DomainEvent represents a business event that has occurred in the domain.
type DomainEvent interface {
	// IsEventType HasEventType returns the string identifier for this event type.
	IsEventType() string

	// HasOccurredAt returns when this event occurred.
	HasOccurredAt() time.Time

	// IsErrorEvent returns true if this event represents an error or failure condition.
	IsErrorEvent() bool
}

// DomainEvents is a slice of DomainEvent instances.
type DomainEvents = []DomainEvent

// MessageID represents a unique message identifier.
type MessageID = string

// CausationID represents the ID of the event that caused this event.
type CausationID = string

// CorrelationID represents the ID correlating related events.
type CorrelationID = string

// EventMetadata contains event tracking information.
type EventMetadata struct {
	MessageID     MessageID
	CausationID   CausationID
	CorrelationID CorrelationID
}

// BuildEventMetadata creates EventMetadata from UUID values.
func BuildEventMetadata(messageID uuid.UUID, causationID uuid.UUID, correlationID uuid.UUID) EventMetadata {
	return EventMetadata{
		MessageID:     messageID.String(),
		CausationID:   causationID.String(),
		CorrelationID: correlationID.String(),
	}
}

// EventMetadataFrom extracts EventMetadata from a StorableEvent.
func EventMetadataFrom(storableEvent eventstore.StorableEvent) (EventMetadata, error) {
	metadata := new(EventMetadata)
	err := jsoniter.ConfigFastest.Unmarshal(storableEvent.MetadataJSON, metadata)
	if err != nil {
		return EventMetadata{}, errors.Join(ErrMappingToEventMetadataFailed, err)
	}

	return *metadata, nil
}
