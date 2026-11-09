package playback

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var ErrInvalid = errors.New("invalid playback access request")

type Store interface {
	IsTrackPlayable(context.Context, uuid.UUID, time.Time) (bool, error)
}

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) *Service {
	return &Service{store: store, now: time.Now}
}

func (service *Service) IsTrackPlayable(
	ctx context.Context,
	trackID uuid.UUID,
) (bool, error) {
	if trackID == uuid.Nil {
		return false, fmt.Errorf("release_track_id must not be nil: %w", ErrInvalid)
	}
	allowed, err := service.store.IsTrackPlayable(ctx, trackID, service.now().UTC())
	if err != nil {
		return false, fmt.Errorf("check track playback access: %w", err)
	}
	return allowed, nil
}
