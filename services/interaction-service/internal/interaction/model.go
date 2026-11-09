package interaction

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidArgument    = errors.New("invalid interaction argument")
	ErrTrackNotFound      = errors.New("catalog track not found")
	ErrAlbumNotFound      = errors.New("catalog album not found")
	ErrCatalogUnavailable = errors.New("catalog service unavailable")
	ErrCatalogTimeout     = errors.New("catalog request timed out")
)

type Like struct {
	UserID    uuid.UUID
	TargetID  uuid.UUID
	CreatedAt time.Time
}

type PageRequest struct {
	Limit  int
	Offset int
}

type LikePage struct {
	Items  []Like
	Limit  int
	Offset int
	Total  int64
}

func invalidArgument(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidArgument, message)
}
