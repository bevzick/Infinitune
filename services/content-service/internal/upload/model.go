package upload

import (
	"errors"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/google/uuid"
)

const MaximumAudioSizeBytes int64 = 500 * 1024 * 1024

var (
	ErrInvalid             = errors.New("invalid upload session")
	ErrNotFound            = errors.New("upload session not found")
	ErrConflict            = errors.New("upload session changed concurrently")
	ErrIdempotencyConflict = errors.New("idempotency key was reused with different upload metadata")
	ErrReleaseLocked       = errors.New("release does not accept uploads")
	ErrInvalidTransition   = errors.New("invalid upload status transition")
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusUploading Status = "uploading"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

type Session struct {
	ID                uuid.UUID
	ReleaseTrackID    uuid.UUID
	RequestedByUserID uuid.UUID
	IdempotencyKey    uuid.UUID
	ObjectKey         string
	OriginalName      string
	MIMEType          string
	SizeBytes         int64
	Status            Status
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type TrackTarget struct {
	TrackID       uuid.UUID
	ReleaseID     uuid.UUID
	CreatorID     uuid.UUID
	ReleaseStatus release.Status
}

type CreateInput struct {
	ReleaseTrackID uuid.UUID
	IdempotencyKey uuid.UUID
	OriginalName   string
	MIMEType       string
	SizeBytes      int64
}

type ValidationError struct {
	Field  string
	Reason string
}

func (err *ValidationError) Error() string { return err.Field + ": " + err.Reason }
func (err *ValidationError) Unwrap() error { return ErrInvalid }
