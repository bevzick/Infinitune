package events

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/media-service/internal/media"
	"github.com/google/uuid"
)

func TestNewAssetReadyBuildsVersionedEvent(t *testing.T) {
	t.Parallel()
	durationMS := 125000
	asset := media.Asset{
		ID: uuid.New(), SourceUploadID: uuid.New(), ContentTrackID: uuid.New(),
		DurationMS: &durationMS,
	}
	variants := []media.AudioVariant{{Codec: "mp3", Container: "mp3", BitrateKbps: 320, SampleRate: 48000, Channels: 2}}
	occurredAt := time.Date(2026, time.November, 12, 12, 0, 0, 0, time.UTC)
	message, err := NewAssetReady(asset, variants, occurredAt, "media-ready")
	if err != nil {
		t.Fatalf("NewAssetReady() error = %v", err)
	}
	if message.AggregateID != asset.ID || message.Key != asset.ID.String() ||
		message.Type != AssetReadyV1 || message.Topic != "media-ready" ||
		!message.OccurredAt.Equal(occurredAt) {
		t.Fatalf("message = %+v", message)
	}
	var envelope Envelope[AssetReadyData]
	if err = json.Unmarshal(message.Payload, &envelope); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if envelope.Source != "media-service" || envelope.Data.MediaAssetID != asset.ID ||
		envelope.Data.SourceUploadID != asset.SourceUploadID ||
		envelope.Data.ContentTrackID == nil || *envelope.Data.ContentTrackID != asset.ContentTrackID ||
		envelope.Data.CatalogTrackID != nil || envelope.Data.DurationMS != durationMS ||
		len(envelope.Data.Variants) != 1 || envelope.Data.Variants[0].BitrateKbps != 320 {
		t.Fatalf("envelope = %+v", envelope)
	}
}
