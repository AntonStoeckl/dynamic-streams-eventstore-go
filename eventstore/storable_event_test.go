package eventstore

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Test_BuildStorableEvent_ErrorCases is a comprehensive test covering multiple error scenarios and edge cases.
// High line count is acceptable for thorough validation of error handling logic.
//
//nolint:fun
//nolint:funlen
func Test_BuildStorableEvent_ErrorCases(t *testing.T) {
	validTime := time.Now()
	validPayloadJSON := []byte(`{"key": "value"}`)
	validMetadataJSON := []byte(`{"meta": "data"}`)

	tests := []struct {
		name         string
		eventType    string
		occurredAt   time.Time
		payloadJSON  []byte
		metadataJSON []byte
		expectedErr  error
	}{
		{
			name:         "invalid payload JSON",
			eventType:    "TestEvent",
			occurredAt:   validTime,
			payloadJSON:  []byte(`{"invalid": json}`),
			metadataJSON: validMetadataJSON,
			expectedErr:  ErrInvalidPayloadJSON,
		},
		{
			name:         "invalid metadata JSON",
			eventType:    "TestEvent",
			occurredAt:   validTime,
			payloadJSON:  validPayloadJSON,
			metadataJSON: []byte(`{"invalid": json}`),
			expectedErr:  ErrInvalidMetadataJSON,
		},
		{
			name:         "empty payload JSON",
			eventType:    "TestEvent",
			occurredAt:   validTime,
			payloadJSON:  []byte(``),
			metadataJSON: validMetadataJSON,
			expectedErr:  ErrInvalidPayloadJSON,
		},
		{
			name:         "empty metadata JSON",
			eventType:    "TestEvent",
			occurredAt:   validTime,
			payloadJSON:  validPayloadJSON,
			metadataJSON: []byte(``),
			expectedErr:  ErrInvalidMetadataJSON,
		},
		{
			name:         "nil payload JSON",
			eventType:    "TestEvent",
			occurredAt:   validTime,
			payloadJSON:  nil,
			metadataJSON: validMetadataJSON,
			expectedErr:  ErrInvalidPayloadJSON,
		},
		{
			name:         "nil metadata JSON",
			eventType:    "TestEvent",
			occurredAt:   validTime,
			payloadJSON:  validPayloadJSON,
			metadataJSON: nil,
			expectedErr:  ErrInvalidMetadataJSON,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BuildStorableEvent(tt.eventType, tt.occurredAt, tt.payloadJSON, tt.metadataJSON)
			assert.ErrorContains(t, err, tt.expectedErr.Error())
		})
	}
}

func Test_BuildStorableEventWithEmptyMetadata_ErrorCases(t *testing.T) {
	validTime := time.Now()

	tests := []struct {
		name        string
		eventType   string
		occurredAt  time.Time
		payloadJSON []byte
		expectedErr error
	}{
		{
			name:        "invalid payload JSON",
			eventType:   "TestEvent",
			occurredAt:  validTime,
			payloadJSON: []byte(`{"invalid": json}`),
			expectedErr: ErrInvalidPayloadJSON,
		},
		{
			name:        "empty payload JSON",
			eventType:   "TestEvent",
			occurredAt:  validTime,
			payloadJSON: []byte(``),
			expectedErr: ErrInvalidPayloadJSON,
		},
		{
			name:        "nil payload JSON",
			eventType:   "TestEvent",
			occurredAt:  validTime,
			payloadJSON: nil,
			expectedErr: ErrInvalidPayloadJSON,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BuildStorableEventWithEmptyMetadata(tt.eventType, tt.occurredAt, tt.payloadJSON)
			assert.ErrorContains(t, err, tt.expectedErr.Error())
		})
	}
}

func Test_BuildStorableEvent_Success(t *testing.T) {
	eventType := "BookCopyLentToReader"
	occurredAt := time.Now()
	payloadJSON := []byte(`{"BookID": "book-123", "ReaderID": "reader-456"}`)
	metadataJSON := []byte(`{"correlationId": "corr-789"}`)

	storableEvent, err := BuildStorableEvent(eventType, occurredAt, payloadJSON, metadataJSON)
	assert.NoError(t, err)
	assert.Equal(t, eventType, storableEvent.EventType)
	assert.Equal(t, occurredAt, storableEvent.OccurredAt)
	assert.Equal(t, payloadJSON, storableEvent.PayloadJSON)
	assert.Equal(t, metadataJSON, storableEvent.MetadataJSON)
}

func Test_BuildStorableEventWithEmptyMetadata_Success(t *testing.T) {
	eventType := "BookCopyReturnedByReader"
	occurredAt := time.Now()
	payloadJSON := []byte(`{"BookID": "book-123", "ReaderID": "reader-456"}`)

	storableEvent, err := BuildStorableEventWithEmptyMetadata(eventType, occurredAt, payloadJSON)
	assert.NoError(t, err)
	assert.Equal(t, eventType, storableEvent.EventType)
	assert.Equal(t, occurredAt, storableEvent.OccurredAt)
	assert.Equal(t, payloadJSON, storableEvent.PayloadJSON)
	assert.Equal(t, []byte(`{}`), storableEvent.MetadataJSON)
}
