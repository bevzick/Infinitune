package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const AssetReadyV1 = "media.asset.ready.v1"

var ErrInvalidEvent = errors.New("invalid event")

type mediaEnvelope struct {
	ID         uuid.UUID      `json:"id"`
	Type       string         `json:"type"`
	Source     string         `json:"source"`
	OccurredAt time.Time      `json:"occurred_at"`
	Data       assetReadyData `json:"data"`
}

type audioVariant struct {
	Codec       string `json:"codec"`
	Container   string `json:"container"`
	BitrateKbps int    `json:"bitrate_kbps"`
	SampleRate  int    `json:"sample_rate"`
	Channels    int    `json:"channels"`
}

type assetReadyData struct {
	MediaAssetID   uuid.UUID      `json:"media_asset_id"`
	SourceUploadID uuid.UUID      `json:"source_upload_id"`
	ContentTrackID *uuid.UUID     `json:"content_track_id"`
	CatalogTrackID *uuid.UUID     `json:"catalog_track_id"`
	DurationMS     int            `json:"duration_ms"`
	Variants       []audioVariant `json:"variants"`
}

type ReadyAsset struct {
	MediaAssetID   uuid.UUID
	SourceUploadID uuid.UUID
	ContentTrackID uuid.UUID
	DurationMS     int
	EventID        uuid.UUID
	OccurredAt     time.Time
}

type MediaReadinessStore interface {
	UpsertReady(context.Context, ReadyAsset) error
}

type MediaHandler struct{ store MediaReadinessStore }

func NewMediaHandler(store MediaReadinessStore) *MediaHandler { return &MediaHandler{store: store} }

func (handler *MediaHandler) Handle(ctx context.Context, payload []byte) error {
	var event mediaEnvelope
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("%w: decode media event: %v", ErrInvalidEvent, err)
	}
	if event.ID == uuid.Nil || event.Type != AssetReadyV1 || event.Source != "media-service" ||
		event.OccurredAt.IsZero() || event.Data.MediaAssetID == uuid.Nil ||
		event.Data.SourceUploadID == uuid.Nil || event.Data.DurationMS <= 0 ||
		!validTrackReference(event.Data.ContentTrackID, event.Data.CatalogTrackID) ||
		!validVariants(event.Data.Variants) {
		return fmt.Errorf("%w: incomplete media asset ready event", ErrInvalidEvent)
	}
	// Catalog-owned media is valid on the shared topic but does not belong in Content's projection.
	if event.Data.ContentTrackID == nil {
		return nil
	}
	asset := ReadyAsset{
		MediaAssetID: event.Data.MediaAssetID, SourceUploadID: event.Data.SourceUploadID,
		ContentTrackID: *event.Data.ContentTrackID, DurationMS: event.Data.DurationMS,
		EventID: event.ID, OccurredAt: event.OccurredAt.UTC(),
	}
	if err := handler.store.UpsertReady(ctx, asset); err != nil {
		return fmt.Errorf("project media asset readiness: %w", err)
	}
	return nil
}

func validTrackReference(contentID, catalogID *uuid.UUID) bool {
	contentValid := contentID != nil && *contentID != uuid.Nil
	catalogValid := catalogID != nil && *catalogID != uuid.Nil
	return contentValid != catalogValid
}

func validVariants(variants []audioVariant) bool {
	if len(variants) == 0 {
		return false
	}
	for _, variant := range variants {
		if variant.Codec == "" || variant.Container == "" || variant.BitrateKbps <= 0 ||
			variant.SampleRate <= 0 || variant.Channels <= 0 {
			return false
		}
	}
	return true
}
