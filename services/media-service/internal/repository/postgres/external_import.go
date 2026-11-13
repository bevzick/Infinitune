package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/bevzick/Infinitune/services/media-service/internal/media"
	"github.com/bevzick/Infinitune/services/media-service/internal/processing"
	"github.com/bevzick/Infinitune/services/media-service/internal/source"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (repository *MediaRepository) ExternalByContentTrack(
	ctx context.Context,
	contentTrackID uuid.UUID,
) (media.Asset, source.Reference, error) {
	if contentTrackID == uuid.Nil {
		return media.Asset{}, source.Reference{}, media.ErrInvalid
	}
	asset, err := scanAsset(repository.database.QueryRow(ctx, `
		SELECT id, track_id, content_track_id, source_upload_id,
		       original_object_key, original_filename, duration_ms,
		       status, created_at, updated_at
		FROM media.media_assets
		WHERE content_track_id = $1 AND source_upload_id IS NULL
	`, contentTrackID))
	if errors.Is(err, pgx.ErrNoRows) {
		return media.Asset{}, source.Reference{}, media.ErrNotFound
	}
	if err != nil {
		return media.Asset{}, source.Reference{}, fmt.Errorf("query external media asset: %w", err)
	}
	record, err := NewSourceRepository(repository.database).ByMediaAsset(ctx, asset.ID)
	if err != nil {
		return media.Asset{}, source.Reference{}, fmt.Errorf("query external media source: %w", err)
	}
	return asset, record.Reference, nil
}

func (repository *MediaRepository) CreateExternal(
	ctx context.Context,
	asset media.Asset,
	reference source.Reference,
) (media.Asset, bool, error) {
	if asset.ID == uuid.Nil || asset.ContentTrackID == uuid.Nil ||
		asset.SourceUploadID != uuid.Nil || strings.TrimSpace(asset.OriginalObjectKey) == "" ||
		strings.TrimSpace(asset.OriginalFilename) == "" || asset.Status != media.StatusUploaded {
		return media.Asset{}, false, fmt.Errorf("invalid external media asset: %w", media.ErrInvalid)
	}
	if err := source.ValidateReference(reference); err != nil {
		return media.Asset{}, false, err
	}
	if reference.Kind != source.KindExternalProvider {
		return media.Asset{}, false, fmt.Errorf("external provider source is required: %w", source.ErrInvalid)
	}
	metadata, err := json.Marshal(reference.Metadata)
	if err != nil {
		return media.Asset{}, false, fmt.Errorf("encode external source metadata: %w", err)
	}
	tx, err := repository.database.Begin(ctx)
	if err != nil {
		return media.Asset{}, false, fmt.Errorf("begin external import: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	stored, err := scanAsset(tx.QueryRow(ctx, `
		INSERT INTO media.media_assets (
			id, track_id, content_track_id, source_upload_id,
			original_object_key, original_filename, status
		)
		VALUES ($1, NULL, $2, NULL, $3, $4, $5)
		ON CONFLICT (content_track_id) WHERE source_upload_id IS NULL DO NOTHING
		RETURNING id, track_id, content_track_id, source_upload_id,
		          original_object_key, original_filename, duration_ms,
		          status, created_at, updated_at
	`, asset.ID, asset.ContentTrackID, asset.OriginalObjectKey,
		asset.OriginalFilename, asset.Status))
	if errors.Is(err, pgx.ErrNoRows) {
		existing, existingReference, findErr := repository.ExternalByContentTrack(ctx, asset.ContentTrackID)
		if findErr != nil {
			return media.Asset{}, false, fmt.Errorf("find idempotent external import: %w", findErr)
		}
		if !sameSourceReference(existingReference, reference) {
			return media.Asset{}, false, source.ErrConflict
		}
		return existing, false, nil
	}
	if err != nil {
		return media.Asset{}, false, fmt.Errorf("insert external media asset: %w", err)
	}
	provider, externalID, sourceURL, licenseName, licenseURL, attribution,
		allowsStreaming, allowsRedistribution := sourceColumns(reference)
	_, err = tx.Exec(ctx, `
		INSERT INTO media.asset_sources (
			media_asset_id, source_kind, provider, external_id, source_url,
			license_name, license_url, attribution, allows_streaming,
			allows_redistribution, metadata
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	`, stored.ID, reference.Kind, provider, externalID, sourceURL,
		licenseName, licenseURL, attribution, allowsStreaming, allowsRedistribution, metadata)
	if err != nil {
		return media.Asset{}, false, fmt.Errorf("insert external media source: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO media.processing_jobs (
			id, media_asset_id, job_type, status, available_at
		) VALUES ($1, $2, $3, 'pending', now())
	`, uuid.New(), stored.ID, processing.JobTypeAudioVariants)
	if err != nil {
		return media.Asset{}, false, fmt.Errorf("enqueue external media processing: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return media.Asset{}, false, fmt.Errorf("commit external import: %w", err)
	}
	return stored, true, nil
}
