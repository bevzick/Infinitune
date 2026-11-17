package artistimage

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

const MaximumBytes int64 = 10 * 1024 * 1024

var (
	ErrInvalid     = errors.New("invalid artist image")
	ErrTooLarge    = errors.New("artist image exceeds 10 MiB")
	ErrForbidden   = errors.New("artist image upload forbidden")
	ErrNotFound    = errors.New("artist image not found")
	ErrStorage     = errors.New("artist image storage failed")
	ErrUnavailable = errors.New("artist image service unavailable")
)

type Image struct {
	CreatorID, CatalogArtistID, UpdatedByUserID uuid.UUID
	ObjectKey, MIMEType                         string
	SizeBytes                                   int64
	CreatedAt, UpdatedAt                        time.Time
}
