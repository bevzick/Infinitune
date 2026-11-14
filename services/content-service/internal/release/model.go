package release

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalid            = errors.New("invalid release")
	ErrNotFound           = errors.New("release not found")
	ErrConflict           = errors.New("release cannot be changed in its current status")
	ErrCatalogUnavailable = errors.New("catalog archival is unavailable")
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
	StatusDraft            Status = "draft"
	StatusUploading        Status = "uploading"
	StatusProcessing       Status = "processing"
	StatusPendingReview    Status = "pending_review"
	StatusApproved         Status = "approved"
	StatusRejected         Status = "rejected"
	StatusChangesRequested Status = "changes_requested"
	StatusPublished        Status = "published"
	StatusArchived         Status = "archived"
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
	ReviewedAt     *time.Time
	PublishedAt    *time.Time
	ArchivedAt     *time.Time
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

type PageRequest struct {
	CreatorID *uuid.UUID
	Status    *Status
	Limit     int
	Offset    int
}

type Page struct {
	Items  []Release
	Limit  int
	Offset int
	Total  int64
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

type UpdateInput struct {
	Title          string
	Type           Type
	ReleaseDate    time.Time
	CoverObjectKey *string
	Tracks         []TrackUpdateInput
}

type TrackUpdateInput struct {
	ID          uuid.UUID
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
