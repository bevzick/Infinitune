//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/events"
	contentmedia "github.com/bevzick/Infinitune/services/content-service/internal/media"
	"github.com/google/uuid"
)

func TestMediaReadinessRepositoryIsIdempotentAndRejectsLateState(t *testing.T) {
	pool := openIntegrationDatabase(t)
	repository := NewMediaReadinessRepository(pool)
	ctx := context.Background()
	assetID, uploadID, trackID := uuid.New(), uuid.New(), uuid.New()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM content.media_asset_readiness WHERE media_asset_id=$1`, assetID)
	})
	newer := events.ReadyAsset{
		MediaAssetID: assetID, SourceUploadID: uploadID, ContentTrackID: trackID,
		DurationMS: 125000, EventID: uuid.New(),
		OccurredAt: time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC),
	}
	if err := repository.UpsertReady(ctx, newer); err != nil {
		t.Fatalf("UpsertReady() error = %v", err)
	}
	if err := repository.UpsertReady(ctx, newer); err != nil {
		t.Fatalf("UpsertReady(duplicate) error = %v", err)
	}
	older := newer
	older.EventID = uuid.New()
	older.DurationMS = 1000
	older.OccurredAt = newer.OccurredAt.Add(-time.Minute)
	if err := repository.UpsertReady(ctx, older); err != nil {
		t.Fatalf("UpsertReady(older) error = %v", err)
	}
	asset, err := repository.AssetByUpload(ctx, uploadID)
	if err != nil || asset.ID != assetID || asset.ContentTrackID != trackID ||
		asset.DurationMS != newer.DurationMS || asset.Status != contentmedia.StatusReady {
		t.Fatalf("AssetByUpload() = %+v, %v", asset, err)
	}
	_, err = repository.AssetByUpload(ctx, uuid.New())
	if !errors.Is(err, contentmedia.ErrNotFound) {
		t.Fatalf("AssetByUpload(missing) error = %v", err)
	}
}

func TestMediaReadinessRepositoryLoadsExternalAssetByTrack(t *testing.T) {
	pool := openIntegrationDatabase(t)
	repository := NewMediaReadinessRepository(pool)
	ctx := context.Background()
	assetID, trackID := uuid.New(), uuid.New()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM content.media_asset_readiness WHERE media_asset_id=$1`, assetID)
	})
	ready := events.ReadyAsset{
		MediaAssetID: assetID, ContentTrackID: trackID, DurationMS: 125000,
		EventID: uuid.New(), OccurredAt: time.Now().UTC(),
	}
	if err := repository.UpsertReady(ctx, ready); err != nil {
		t.Fatalf("UpsertReady() error = %v", err)
	}
	asset, err := repository.AssetByTrack(ctx, trackID)
	if err != nil || asset.ID != assetID || asset.SourceUploadID != uuid.Nil ||
		asset.ContentTrackID != trackID || asset.Status != contentmedia.StatusReady {
		t.Fatalf("AssetByTrack() = %+v, %v", asset, err)
	}
	if _, err = repository.AssetByTrack(ctx, uuid.New()); !errors.Is(err, contentmedia.ErrNotFound) {
		t.Fatalf("AssetByTrack(missing) error = %v", err)
	}
}
