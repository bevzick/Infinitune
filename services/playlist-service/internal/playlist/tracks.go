package playlist

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

const (
	defaultTrackPageLimit = 50
	maximumTrackPageLimit = 100
)

type TrackStore interface {
	Tracks(context.Context, uuid.UUID, PageRequest) (TrackPage, error)
	AddTrack(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) (Track, error)
	RemoveTrack(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error
	ReorderTracks(context.Context, uuid.UUID, uuid.UUID, []uuid.UUID) ([]Track, error)
}

type TrackValidator interface {
	ValidateTrack(context.Context, uuid.UUID) error
}

type TrackService struct {
	playlists *Service
	store     TrackStore
	validator TrackValidator
}

func NewTrackService(
	playlists *Service,
	store TrackStore,
	validator TrackValidator,
) *TrackService {
	return &TrackService{playlists: playlists, store: store, validator: validator}
}

func (service *TrackService) Tracks(
	ctx context.Context,
	viewerID uuid.UUID,
	playlistID uuid.UUID,
	page PageRequest,
) (TrackPage, error) {
	if _, err := service.playlists.ByID(ctx, viewerID, playlistID); err != nil {
		return TrackPage{}, err
	}
	page, err := validateTrackPage(page)
	if err != nil {
		return TrackPage{}, err
	}
	tracks, err := service.store.Tracks(ctx, playlistID, page)
	if err != nil {
		return TrackPage{}, fmt.Errorf("read playlist tracks: %w", err)
	}
	return tracks, nil
}

func validateTrackPage(page PageRequest) (PageRequest, error) {
	if page.Limit == 0 {
		page.Limit = defaultTrackPageLimit
	}
	if page.Limit < 1 || page.Limit > maximumTrackPageLimit {
		return PageRequest{}, invalidArgument("track page limit must be from 1 to 100")
	}
	if page.Offset < 0 {
		return PageRequest{}, invalidArgument("track page offset must not be negative")
	}
	return page, nil
}

func (service *TrackService) Add(
	ctx context.Context,
	actorID uuid.UUID,
	playlistID uuid.UUID,
	trackID uuid.UUID,
) (Track, error) {
	owned, err := service.manualOwned(ctx, actorID, playlistID)
	if err != nil {
		return Track{}, err
	}
	if trackID == uuid.Nil {
		return Track{}, invalidArgument("track id is required")
	}
	if err = service.validator.ValidateTrack(ctx, trackID); err != nil {
		return Track{}, fmt.Errorf("validate catalog track: %w", err)
	}
	added, err := service.store.AddTrack(ctx, owned.ID, owned.OwnerID, trackID, actorID)
	if err != nil {
		return Track{}, fmt.Errorf("add playlist track: %w", err)
	}
	return added, nil
}

func (service *TrackService) Remove(
	ctx context.Context,
	actorID uuid.UUID,
	playlistID uuid.UUID,
	trackID uuid.UUID,
) error {
	owned, err := service.manualOwned(ctx, actorID, playlistID)
	if err != nil {
		return err
	}
	if trackID == uuid.Nil {
		return invalidArgument("track id is required")
	}
	if err = service.store.RemoveTrack(ctx, owned.ID, owned.OwnerID, trackID); err != nil {
		return fmt.Errorf("remove playlist track: %w", err)
	}
	return nil
}

func (service *TrackService) Reorder(
	ctx context.Context,
	actorID uuid.UUID,
	playlistID uuid.UUID,
	trackIDs []uuid.UUID,
) ([]Track, error) {
	owned, err := service.manualOwned(ctx, actorID, playlistID)
	if err != nil {
		return nil, err
	}
	if err = validateTrackOrder(trackIDs); err != nil {
		return nil, err
	}
	tracks, err := service.store.ReorderTracks(ctx, owned.ID, owned.OwnerID, trackIDs)
	if err != nil {
		return nil, fmt.Errorf("reorder playlist tracks: %w", err)
	}
	return tracks, nil
}

func (service *TrackService) manualOwned(
	ctx context.Context,
	actorID uuid.UUID,
	playlistID uuid.UUID,
) (Playlist, error) {
	owned, err := service.playlists.owned(ctx, actorID, playlistID)
	if err != nil {
		return Playlist{}, err
	}
	if owned.Type != TypeManual {
		return Playlist{}, ErrManualOnly
	}
	return owned, nil
}

func validateTrackOrder(trackIDs []uuid.UUID) error {
	seen := make(map[uuid.UUID]struct{}, len(trackIDs))
	for _, trackID := range trackIDs {
		if trackID == uuid.Nil {
			return invalidArgument("track order must contain UUIDs")
		}
		if _, exists := seen[trackID]; exists {
			return invalidArgument("track order must not contain duplicates")
		}
		seen[trackID] = struct{}{}
	}
	return nil
}
