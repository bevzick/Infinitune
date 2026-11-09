package rights

import (
	"errors"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/google/uuid"
)

var (
	ErrInvalid   = errors.New("invalid track rights")
	ErrNotFound  = errors.New("track rights not found")
	ErrImmutable = errors.New("track rights cannot be changed in current release status")
)

type Type string

const (
	TypeStreaming    Type = "streaming"
	TypeDownload     Type = "download"
	TypeDistribution Type = "distribution"
	TerritoryWorld        = "001"
)

type Right struct {
	ID              uuid.UUID
	ReleaseTrackID  uuid.UUID
	TerritoryCode   string
	Type            Type
	ValidFrom       time.Time
	ValidUntil      *time.Time
	GrantedByUserID uuid.UUID
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type TrackTarget struct {
	TrackID       uuid.UUID
	ReleaseID     uuid.UUID
	CreatorID     uuid.UUID
	ReleaseStatus release.Status
}

type RightTarget struct {
	Right Right
	Track TrackTarget
}

type SetInput struct {
	ReleaseTrackID uuid.UUID
	TerritoryCode  string
	Type           Type
	ValidFrom      time.Time
	ValidUntil     *time.Time
}

type ValidationError struct {
	Field  string
	Reason string
}

func (err *ValidationError) Error() string { return err.Field + ": " + err.Reason }
func (err *ValidationError) Unwrap() error { return ErrInvalid }
