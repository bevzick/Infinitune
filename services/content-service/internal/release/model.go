package release

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalid  = errors.New("invalid release")
	ErrNotFound = errors.New("release not found")
)

type Type string

const (
	TypeSingle      Type = "single"
	TypeEP          Type = "ep"
	TypeAlbum       Type = "album"
	TypeCompilation Type = "compilation"
)

type Status string

const (
	StatusDraft         Status = "draft"
	StatusUploading     Status = "uploading"
	StatusProcessing    Status = "processing"
	StatusPendingReview Status = "pending_review"
	StatusApproved      Status = "approved"
	StatusRejected      Status = "rejected"
	StatusPublished     Status = "published"
)

type Release struct {
	ID             uuid.UUID
	CreatorID      uuid.UUID
	Title          string
	Type           Type
	ReleaseDate    time.Time
	Status         Status
	CoverObjectKey *string
	CatalogAlbumID *uuid.UUID
	SubmittedAt    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Track struct {
	ID             uuid.UUID
	ReleaseID      uuid.UUID
	Title          string
	TrackNumber    int32
	DiscNumber     int32
	Explicit       bool
	CatalogTrackID *uuid.UUID
	CreatedAt      time.Time
}

type Draft struct {
	Release Release
	Tracks  []Track
}

type CreateDraftInput struct {
	CreatorID   uuid.UUID
	Title       string
	Type        Type
	ReleaseDate time.Time
	Tracks      []TrackInput
}

type TrackInput struct {
	Title       string
	TrackNumber int32
	DiscNumber  int32
	Explicit    bool
}

type ValidationError struct {
	Field  string
	Reason string
}

func (err *ValidationError) Error() string { return err.Field + ": " + err.Reason }
func (err *ValidationError) Unwrap() error { return ErrInvalid }
