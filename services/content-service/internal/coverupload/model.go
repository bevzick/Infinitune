package coverupload

import (
	"errors"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/google/uuid"
)

const MaximumCoverSizeBytes int64 = 10 * 1024 * 1024

var (
	ErrInvalid             = errors.New("invalid cover upload session")
	ErrNotFound            = errors.New("cover upload session or release not found")
	ErrConflict            = errors.New("cover upload session changed concurrently")
	ErrIdempotencyConflict = errors.New("idempotency key was reused with different cover metadata")
	ErrReleaseLocked       = errors.New("release does not accept cover uploads")
	ErrInvalidTransition   = errors.New("invalid cover upload status transition")
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
	ReleaseID         uuid.UUID
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

type ReleaseTarget struct {
	ReleaseID uuid.UUID
	CreatorID uuid.UUID
	Status    release.Status
}

type CreateInput struct {
	ReleaseID      uuid.UUID
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
