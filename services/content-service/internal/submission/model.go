package submission

import (
	"errors"
	"fmt"

	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/google/uuid"
)

var (
	ErrInvalid           = errors.New("invalid release submission")
	ErrNotFound          = errors.New("release not found")
	ErrNotReady          = errors.New("release is not ready for moderation")
	ErrConflict          = errors.New("release cannot be submitted in current status")
	ErrMediaUnavailable  = errors.New("media readiness check unavailable")
	ErrInconsistentMedia = errors.New("media asset does not match release track")
)

type Candidate struct {
	Release release.Release
	Tracks  []Track
}

type Track struct {
	ID                       uuid.UUID
	CompletedUploadID        *uuid.UUID
	HasWorldwideStreaming    bool
	HasWorldwideDistribution bool
}

type ReadinessError struct {
	TrackID uuid.UUID
	Reason  string
}

func (err *ReadinessError) Error() string {
	if err.TrackID == uuid.Nil {
		return err.Reason
	}
	return fmt.Sprintf("track %s: %s", err.TrackID, err.Reason)
}

func (err *ReadinessError) Unwrap() error { return ErrNotReady }

type ValidationError struct {
	Field  string
	Reason string
}

func (err *ValidationError) Error() string { return err.Field + ": " + err.Reason }
func (err *ValidationError) Unwrap() error { return ErrInvalid }
