package playlist

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrForbidden          = errors.New("playlist access forbidden")
	ErrInvalidArgument    = errors.New("invalid playlist argument")
	ErrNotFound           = errors.New("playlist not found")
	ErrManualOnly         = errors.New("operation is supported only for manual playlists")
	ErrTrackExists        = errors.New("track already exists in playlist")
	ErrTrackNotFound      = errors.New("playlist track not found")
	ErrCatalogUnavailable = errors.New("catalog service unavailable")
)

type Type string

const (
	TypeManual  Type = "manual"
	TypeDynamic Type = "dynamic"
)

type Visibility string

const (
	VisibilityPrivate  Visibility = "private"
	VisibilityUnlisted Visibility = "unlisted"
	VisibilityPublic   Visibility = "public"
)

type Playlist struct {
	ID          uuid.UUID
	OwnerID     uuid.UUID
	Name        string
	Description string
	Type        Type
	Visibility  Visibility
	RuleSetID   *uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type CreateInput struct {
	Name        string
	Description string
	Visibility  Visibility
}

type UpdateInput struct {
	Name        *string
	Description *string
	Visibility  *Visibility
}

type Track struct {
	PlaylistID uuid.UUID
	TrackID    uuid.UUID
	Position   int
	AddedBy    uuid.UUID
	AddedAt    time.Time
}

type PageRequest struct {
	Limit  int
	Offset int
}

type TrackPage struct {
	Items  []Track
	Limit  int
	Offset int
	Total  int64
}
