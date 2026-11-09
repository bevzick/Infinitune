package publication

import (
	"errors"
	"time"

	contentcatalog "github.com/bevzick/Infinitune/services/content-service/internal/catalog"
	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/google/uuid"
)

var (
	ErrInvalid             = errors.New("invalid release publication")
	ErrNotFound            = errors.New("release not found")
	ErrConflict            = errors.New("release cannot be published in its current status")
	ErrNotReady            = errors.New("release is not ready for publication")
	ErrMediaUnavailable    = errors.New("media service unavailable")
	ErrCatalogUnavailable  = errors.New("catalog service unavailable")
	ErrInconsistentMedia   = errors.New("release media association is inconsistent")
	ErrInconsistentCatalog = errors.New("catalog publication response is inconsistent")
)

type Candidate struct {
	Release     release.Release
	CreatorName string
	Tracks      []Track
}

type Track struct {
	release.Track
	CompletedUploadID *uuid.UUID
}

type Record struct {
	ReleaseID   uuid.UUID
	AlbumID     *uuid.UUID
	Tracks      []contentcatalog.PublishedTrack
	PublishedAt time.Time
}

type ValidationError struct {
	Field  string
	Reason string
}

func (err *ValidationError) Error() string { return err.Field + ": " + err.Reason }
func (err *ValidationError) Unwrap() error { return ErrInvalid }

type ReadinessError struct {
	TrackID uuid.UUID
	Reason  string
}

func (err *ReadinessError) Error() string {
	if err.TrackID == uuid.Nil {
		return err.Reason
	}
	return "track " + err.TrackID.String() + ": " + err.Reason
}

func (err *ReadinessError) Unwrap() error { return ErrNotReady }
