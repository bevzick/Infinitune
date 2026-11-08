package playlist

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrForbidden       = errors.New("playlist access forbidden")
	ErrInvalidArgument = errors.New("invalid playlist argument")
	ErrNotFound        = errors.New("playlist not found")
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
