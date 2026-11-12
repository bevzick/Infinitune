package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	ReleasePublishedV1 = "catalog.release.published.v1"
	ReleaseArchivedV1  = "catalog.release.archived.v1"
)

type catalogEnvelope struct {
	ID         uuid.UUID       `json:"id"`
	Type       string          `json:"type"`
	Source     string          `json:"source"`
	OccurredAt time.Time       `json:"occurred_at"`
	Data       json.RawMessage `json:"data"`
}

type releasePublishedData struct {
	SourceReleaseID uuid.UUID `json:"source_release_id"`
}

type releaseArchivedData struct {
	SourceReleaseID uuid.UUID   `json:"source_release_id"`
	TrackIDs        []uuid.UUID `json:"track_ids"`
}

type CatalogRunInvalidator interface {
	InvalidateAllBefore(context.Context, time.Time) (int64, error)
	InvalidateTracksBefore(context.Context, []uuid.UUID, time.Time) (int64, error)
}

type CatalogHandler struct {
	runs CatalogRunInvalidator
}

func NewCatalogHandler(runs CatalogRunInvalidator) *CatalogHandler {
	return &CatalogHandler{runs: runs}
}

func (handler *CatalogHandler) Handle(ctx context.Context, payload []byte) error {
	var event catalogEnvelope
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("%w: decode catalog event: %v", ErrInvalidEvent, err)
	}
	if event.ID == uuid.Nil || event.Source != "catalog-service" || event.OccurredAt.IsZero() {
		return fmt.Errorf("%w: incomplete catalog event", ErrInvalidEvent)
	}
	switch event.Type {
	case ReleasePublishedV1:
		var data releasePublishedData
		if err := json.Unmarshal(event.Data, &data); err != nil || data.SourceReleaseID == uuid.Nil {
			return fmt.Errorf("%w: invalid release published data", ErrInvalidEvent)
		}
		if _, err := handler.runs.InvalidateAllBefore(ctx, event.OccurredAt); err != nil {
			return fmt.Errorf("invalidate runs after catalog publication: %w", err)
		}
	case ReleaseArchivedV1:
		var data releaseArchivedData
		if err := json.Unmarshal(event.Data, &data); err != nil ||
			data.SourceReleaseID == uuid.Nil || !validTrackIDs(data.TrackIDs) {
			return fmt.Errorf("%w: invalid release archived data", ErrInvalidEvent)
		}
		if _, err := handler.runs.InvalidateTracksBefore(
			ctx, data.TrackIDs, event.OccurredAt,
		); err != nil {
			return fmt.Errorf("invalidate runs after catalog archive: %w", err)
		}
	default:
		return fmt.Errorf("%w: unsupported catalog event type", ErrInvalidEvent)
	}
	return nil
}

func validTrackIDs(ids []uuid.UUID) bool {
	if len(ids) == 0 {
		return false
	}
	seen := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			return false
		}
		if _, exists := seen[id]; exists {
			return false
		}
		seen[id] = struct{}{}
	}
	return true
}
