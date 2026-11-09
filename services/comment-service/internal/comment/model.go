package comment

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidArgument    = errors.New("invalid comment argument")
	ErrNotFound           = errors.New("comment not found")
	ErrForbidden          = errors.New("comment access forbidden")
	ErrNotEditable        = errors.New("comment is not editable")
	ErrParentInvalid      = errors.New("comment parent is invalid")
	ErrRateLimited        = errors.New("comment rate limit exceeded")
	ErrTrackNotFound      = errors.New("catalog track not found")
	ErrCatalogUnavailable = errors.New("catalog service unavailable")
	ErrCatalogTimeout     = errors.New("catalog request timed out")
)

type Status string

const (
	StatusPublished Status = "published"
	StatusHidden    Status = "hidden"
	StatusDeleted   Status = "deleted"
)

type Comment struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TrackID   uuid.UUID
	ParentID  *uuid.UUID
	Content   string
	Status    Status
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

type CreateInput struct {
	TrackID  uuid.UUID
	ParentID *uuid.UUID
	Content  string
}

type PageRequest struct {
	Limit  int
	Offset int
}

type Page struct {
	Items  []Comment
	Limit  int
	Offset int
	Total  int64
}

func invalidArgument(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidArgument, message)
}
