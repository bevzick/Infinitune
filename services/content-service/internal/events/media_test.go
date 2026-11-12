package events

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMediaHandlerProjectsCreatorAsset(t *testing.T) {
	store := &mediaStoreStub{}
	eventID, assetID, uploadID, trackID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	payload := []byte(`{"id":"` + eventID.String() + `","type":"media.asset.ready.v1",` +
		`"source":"media-service","occurred_at":"2026-11-12T12:00:00Z","data":{` +
		`"media_asset_id":"` + assetID.String() + `","source_upload_id":"` + uploadID.String() + `",` +
		`"content_track_id":"` + trackID.String() + `","duration_ms":125000,` +
		`"variants":[{"codec":"mp3","container":"mp3","bitrate_kbps":320,"sample_rate":48000,"channels":2}]}}`)
	if err := NewMediaHandler(store).Handle(context.Background(), payload); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if store.asset.EventID != eventID || store.asset.MediaAssetID != assetID ||
		store.asset.SourceUploadID != uploadID || store.asset.ContentTrackID != trackID ||
		store.asset.DurationMS != 125000 ||
		!store.asset.OccurredAt.Equal(time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("asset = %+v", store.asset)
	}
}

func TestMediaHandlerAcknowledgesCatalogAssetWithoutProjecting(t *testing.T) {
	store := &mediaStoreStub{}
	payload := []byte(`{"id":"` + uuid.NewString() + `","type":"media.asset.ready.v1",` +
		`"source":"media-service","occurred_at":"2026-11-12T12:00:00Z","data":{` +
		`"media_asset_id":"` + uuid.NewString() + `","source_upload_id":"` + uuid.NewString() + `",` +
		`"catalog_track_id":"` + uuid.NewString() + `","duration_ms":125000,` +
		`"variants":[{"codec":"mp3","container":"mp3","bitrate_kbps":320,"sample_rate":48000,"channels":2}]}}`)
	if err := NewMediaHandler(store).Handle(context.Background(), payload); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if store.calls != 0 {
		t.Fatalf("UpsertReady calls = %d", store.calls)
	}
}

func TestMediaHandlerRejectsMalformedAsset(t *testing.T) {
	payload := []byte(`{"id":"` + uuid.NewString() + `","type":"media.asset.ready.v1",` +
		`"source":"media-service","occurred_at":"2026-11-12T12:00:00Z","data":{` +
		`"media_asset_id":"` + uuid.NewString() + `","source_upload_id":"` + uuid.NewString() + `",` +
		`"content_track_id":"` + uuid.NewString() + `","duration_ms":0,"variants":[]}}`)
	err := NewMediaHandler(&mediaStoreStub{}).Handle(context.Background(), payload)
	if !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("Handle() error = %v, want %v", err, ErrInvalidEvent)
	}
}

type mediaStoreStub struct {
	asset ReadyAsset
	calls int
}

func (stub *mediaStoreStub) UpsertReady(_ context.Context, asset ReadyAsset) error {
	stub.asset = asset
	stub.calls++
	return nil
}
