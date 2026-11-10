package history

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	maximumSubmissionAge = 24 * time.Hour
	allowedClockSkew     = 30 * time.Second
	playbackTimeLeeway   = 5 * time.Second
)

type EventStore interface {
	Record(context.Context, Event) (Event, error)
}

type TrackCatalog interface {
	Duration(context.Context, uuid.UUID) (int, error)
}

type Service struct {
	store   EventStore
	catalog TrackCatalog
	now     func() time.Time
}

func NewService(store EventStore, catalog TrackCatalog) *Service {
	return &Service{store: store, catalog: catalog, now: time.Now}
}

func (service *Service) Record(
	ctx context.Context,
	userID uuid.UUID,
	input RecordInput,
) (RecordResult, error) {
	now := service.now().UTC()
	if err := validateInput(userID, input, now); err != nil {
		return RecordResult{}, err
	}
	durationMS, err := service.catalog.Duration(ctx, input.TrackID)
	if err != nil {
		return RecordResult{}, err
	}
	if durationMS <= 0 {
		return RecordResult{}, fmt.Errorf("%w: Catalog returned an invalid track duration", ErrCatalogUnavailable)
	}
	if input.ListenedMS > durationMS {
		return RecordResult{}, fmt.Errorf("%w: listened_ms cannot exceed track duration", ErrInvalidArgument)
	}
	elapsed := input.FinishedAt.Sub(input.StartedAt)
	if time.Duration(input.ListenedMS)*time.Millisecond > elapsed+playbackTimeLeeway {
		return RecordResult{}, fmt.Errorf("%w: listened_ms exceeds elapsed playback time", ErrInvalidArgument)
	}
	result := RecordResult{EventID: input.EventID}
	if !Qualifies(input.ListenedMS, durationMS) {
		return result, nil
	}
	result.Recorded = true
	result.Completed = IsCompleted(input.ListenedMS, durationMS)
	finishedAt := input.FinishedAt.UTC()
	_, err = service.store.Record(ctx, Event{
		ID: input.EventID, UserID: userID, TrackID: input.TrackID,
		StartedAt: input.StartedAt.UTC(), FinishedAt: &finishedAt,
		ListenedMS: input.ListenedMS, Completed: result.Completed,
		ContextType: input.ContextType, ContextID: input.ContextID, DeviceType: input.DeviceType,
	})
	if err != nil {
		return RecordResult{}, err
	}
	return result, nil
}

func validateInput(userID uuid.UUID, input RecordInput, now time.Time) error {
	if userID == uuid.Nil || input.EventID == uuid.Nil || input.TrackID == uuid.Nil {
		return fmt.Errorf("%w: event_id and track_id are required", ErrInvalidArgument)
	}
	if input.StartedAt.IsZero() || input.FinishedAt.IsZero() || input.FinishedAt.Before(input.StartedAt) {
		return fmt.Errorf("%w: playback time range is invalid", ErrInvalidArgument)
	}
	if input.FinishedAt.After(now.Add(allowedClockSkew)) || input.StartedAt.Before(now.Add(-maximumSubmissionAge)) {
		return fmt.Errorf("%w: playback time is outside the accepted range", ErrInvalidArgument)
	}
	if input.ListenedMS < 0 {
		return fmt.Errorf("%w: listened_ms cannot be negative", ErrInvalidArgument)
	}
	validDevice := input.DeviceType == "web" || input.DeviceType == "mobile" ||
		input.DeviceType == "desktop" || input.DeviceType == "tv" || input.DeviceType == "other"
	if !validDevice {
		return fmt.Errorf("%w: device_type is unsupported", ErrInvalidArgument)
	}
	requiresContext := input.ContextType == "album" || input.ContextType == "playlist" ||
		input.ContextType == "artist" || input.ContextType == "recommendation"
	withoutContext := input.ContextType == "direct" || input.ContextType == "search"
	if (!requiresContext && !withoutContext) || (requiresContext && input.ContextID == nil) ||
		(withoutContext && input.ContextID != nil) || (input.ContextID != nil && *input.ContextID == uuid.Nil) {
		return fmt.Errorf("%w: playback context is invalid", ErrInvalidArgument)
	}
	return nil
}
