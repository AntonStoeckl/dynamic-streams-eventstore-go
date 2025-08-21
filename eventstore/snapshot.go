package eventstore

import (
	"encoding/json"
	"errors"
	"time"

	jsoniter "github.com/json-iterator/go"
)

var (
	// ErrInvalidSnapshotJSON is returned when snapshot JSON data is malformed or invalid.
	ErrInvalidSnapshotJSON = errors.New("snapshot json is not valid")

	// ErrEmptyProjectionType is returned when an empty projection type is provided.
	ErrEmptyProjectionType = errors.New("projection type must not be empty")

	// ErrEmptyFilterHash is returned when an empty filter hash is provided.
	ErrEmptyFilterHash = errors.New("filter hash must not be empty")

	// ErrSavingSnapshotFailed is returned when the snapshot save operation fails.
	ErrSavingSnapshotFailed = errors.New("saving snapshot failed")

	// ErrLoadingSnapshotFailed is returned when the snapshot load operation fails.
	ErrLoadingSnapshotFailed = errors.New("loading snapshot failed")

	// ErrDeletingSnapshotFailed is returned when the snapshot delete operation fails.
	ErrDeletingSnapshotFailed = errors.New("deleting snapshot failed")
)

// Snapshot represents a stored projection state with metadata for incremental updates.
// It contains the serialized projection data along with the sequence number of the last
// processed event, enabling efficient incremental updates.
type Snapshot struct {
	ProjectionType string                // Type of projection (e.g., "BooksInCirculation")
	FilterHash     string                // Hash of the filter used to create this snapshot
	SequenceNumber MaxSequenceNumberUint // Last processed event sequence number
	Data           json.RawMessage       // Serialized projection state as JSON
	CreatedAt      time.Time             // When this snapshot was created/updated
}

// Validate ensures the snapshot has valid data for storage operations.
func (s Snapshot) Validate() error {
	if s.ProjectionType == "" {
		return ErrEmptyProjectionType
	}

	if s.FilterHash == "" {
		return ErrEmptyFilterHash
	}

	if !jsoniter.ConfigFastest.Valid(s.Data) {
		return ErrInvalidSnapshotJSON
	}

	return nil
}

// BuildSnapshot creates a new Snapshot with validation.
func BuildSnapshot(
	projectionType string,
	filterHash string,
	sequenceNumber MaxSequenceNumberUint,
	data json.RawMessage,
) (Snapshot, error) {
	snapshot := Snapshot{
		ProjectionType: projectionType,
		FilterHash:     filterHash,
		SequenceNumber: sequenceNumber,
		Data:           data,
		CreatedAt:      time.Now(),
	}

	if err := snapshot.Validate(); err != nil {
		return Snapshot{}, err
	}

	return snapshot, nil
}
