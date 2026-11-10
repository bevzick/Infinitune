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
	List(context.Context, uuid.UUID, ListFilter, int) ([]Event, error)
	Delete(context.Context, uuid.UUID, *time.Time) (int64, error)
}

func (service *Service) List(ctx context.Context, userID uuid.UUID, request ListRequest) (Page, error) {
	if userID == uuid.Nil {
		return Page{}, fmt.Errorf("%w: user identity is required", ErrInvalidArgument)
	}
	if err := validateListRequest(request); err != nil {
		return Page{}, err
	}
	limit := request.Limit
	if limit == 0 {
		limit = 25
	}
	items, err := service.store.List(ctx, userID, request.Filter, limit+1)
	if err != nil {
		return Page{}, err
	}
	page := Page{Items: items}
	if len(items) > limit {
		page.Items = items[:limit]
		last := page.Items[len(page.Items)-1]
		page.NextCursor = &Cursor{StartedAt: last.StartedAt, EventID: last.ID}
	}
	return page, nil
}

func (service *Service) Clear(ctx context.Context, userID uuid.UUID, request ClearRequest) (ClearResult, error) {
	if userID == uuid.Nil {
		return ClearResult{}, fmt.Errorf("%w: user identity is required", ErrInvalidArgument)
	}
	if request.Before != nil && (request.Before.IsZero() || request.Before.After(service.now().UTC().Add(allowedClockSkew))) {
		return ClearResult{}, fmt.Errorf("%w: before is invalid", ErrInvalidArgument)
	}
	deleted, err := service.store.Delete(ctx, userID, request.Before)
	if err != nil {
		return ClearResult{}, err
	}
	return ClearResult{Deleted: deleted}, nil
}

func validateListRequest(request ListRequest) error {
	if request.Limit < 0 || request.Limit > 100 {
		return fmt.Errorf("%w: limit must be between 1 and 100 when provided", ErrInvalidArgument)
	}
	filter := request.Filter
	if filter.TrackID != nil && *filter.TrackID == uuid.Nil {
		return fmt.Errorf("%w: track_id is invalid", ErrInvalidArgument)
	}
	if (filter.From != nil && filter.From.IsZero()) || (filter.To != nil && filter.To.IsZero()) {
		return fmt.Errorf("%w: time filter is invalid", ErrInvalidArgument)
	}
	if filter.From != nil && filter.To != nil && !filter.From.Before(*filter.To) {
		return fmt.Errorf("%w: from must be before to", ErrInvalidArgument)
	}
	if filter.ContextType != "" && !validContextType(filter.ContextType) {
		return fmt.Errorf("%w: context_type is unsupported", ErrInvalidArgument)
	}
	if filter.DeviceType != "" && !validDeviceType(filter.DeviceType) {
		return fmt.Errorf("%w: device_type is unsupported", ErrInvalidArgument)
	}
	if filter.Cursor != nil && (filter.Cursor.EventID == uuid.Nil || filter.Cursor.StartedAt.IsZero()) {
		return fmt.Errorf("%w: cursor is invalid", ErrInvalidArgument)
	}
	return nil
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
	if !validDeviceType(input.DeviceType) {
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

func validContextType(value string) bool {
	return value == "direct" || value == "album" || value == "playlist" || value == "artist" || value == "recommendation" || value == "search"
}
func validDeviceType(value string) bool {
	return value == "web" || value == "mobile" || value == "desktop" || value == "tv" || value == "other"
}
