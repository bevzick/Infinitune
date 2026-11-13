package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/bevzick/Infinitune/services/content-service/internal/events"
	contentmedia "github.com/bevzick/Infinitune/services/content-service/internal/media"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MediaReadinessRepository struct{ pool *pgxpool.Pool }

func NewMediaReadinessRepository(pool *pgxpool.Pool) *MediaReadinessRepository {
	return &MediaReadinessRepository{pool: pool}
}

func (repository *MediaReadinessRepository) UpsertReady(ctx context.Context, asset events.ReadyAsset) error {
	var sourceUploadID any
	if asset.SourceUploadID != uuid.Nil {
		sourceUploadID = asset.SourceUploadID
	}
	_, err := repository.pool.Exec(ctx, `
		INSERT INTO content.media_asset_readiness (
			media_asset_id, source_upload_id, content_track_id, duration_ms, event_id, occurred_at
		) VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (media_asset_id) DO UPDATE
		SET source_upload_id=EXCLUDED.source_upload_id,
		    content_track_id=EXCLUDED.content_track_id,
		    duration_ms=EXCLUDED.duration_ms,
		    event_id=EXCLUDED.event_id,
		    occurred_at=EXCLUDED.occurred_at,
		    updated_at=now()
		WHERE EXCLUDED.occurred_at > content.media_asset_readiness.occurred_at
	`, asset.MediaAssetID, sourceUploadID, asset.ContentTrackID,
		asset.DurationMS, asset.EventID, asset.OccurredAt)
	if err != nil {
		return fmt.Errorf("upsert media readiness: %w", err)
	}
	return nil
}

func (repository *MediaReadinessRepository) AssetByUpload(ctx context.Context, uploadID uuid.UUID) (contentmedia.Asset, error) {
	if uploadID == uuid.Nil {
		return contentmedia.Asset{}, fmt.Errorf("upload_id must be a UUID: %w", contentmedia.ErrInvalid)
	}
	var asset contentmedia.Asset
	err := repository.pool.QueryRow(ctx, `
		SELECT media_asset_id, source_upload_id, content_track_id, duration_ms
		FROM content.media_asset_readiness WHERE source_upload_id=$1
	`, uploadID).Scan(&asset.ID, &asset.SourceUploadID, &asset.ContentTrackID, &asset.DurationMS)
	if errors.Is(err, pgx.ErrNoRows) {
		return contentmedia.Asset{}, contentmedia.ErrNotFound
	}
	if err != nil {
		return contentmedia.Asset{}, fmt.Errorf("query media readiness: %w", err)
	}
	asset.Status = contentmedia.StatusReady
	return asset, nil
}
