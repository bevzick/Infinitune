package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/bevzick/Infinitune/services/media-service/internal/media"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type mediaDatabase interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type MediaRepository struct {
	database mediaDatabase
}

var _ media.Store = (*MediaRepository)(nil)

func NewMediaRepository(database mediaDatabase) *MediaRepository {
	return &MediaRepository{database: database}
}

func (repository *MediaRepository) AssetByID(
	ctx context.Context,
	id uuid.UUID,
) (media.Asset, error) {
	var asset media.Asset
	err := repository.database.QueryRow(ctx, `
		SELECT
			id, track_id, source_upload_id, original_object_key, original_filename,
			duration_ms, status, created_at, updated_at
		FROM media.media_assets
		WHERE id = $1
	`, id).Scan(
		&asset.ID,
		&asset.TrackID,
		&asset.SourceUploadID,
		&asset.OriginalObjectKey,
		&asset.OriginalFilename,
		&asset.DurationMS,
		&asset.Status,
		&asset.CreatedAt,
		&asset.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return media.Asset{}, media.ErrNotFound
	}
	if err != nil {
		return media.Asset{}, fmt.Errorf("query media asset: %w", err)
	}

	asset.Variants, err = repository.audioVariants(ctx, id)
	if err != nil {
		return media.Asset{}, fmt.Errorf("query audio variants: %w", err)
	}
	return asset, nil
}

func (repository *MediaRepository) audioVariants(
	ctx context.Context,
	mediaAssetID uuid.UUID,
) ([]media.AudioVariant, error) {
	rows, err := repository.database.Query(ctx, `
		SELECT
			id, object_key, codec, container, bitrate_kbps, sample_rate,
			channels, size_bytes, created_at
		FROM media.audio_variants
		WHERE media_asset_id = $1
		ORDER BY bitrate_kbps, id
	`, mediaAssetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	variants := make([]media.AudioVariant, 0)
	for rows.Next() {
		var variant media.AudioVariant
		if err = rows.Scan(
			&variant.ID,
			&variant.ObjectKey,
			&variant.Codec,
			&variant.Container,
			&variant.BitrateKbps,
			&variant.SampleRate,
			&variant.Channels,
			&variant.SizeBytes,
			&variant.CreatedAt,
		); err != nil {
			return nil, err
		}
		variants = append(variants, variant)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return variants, nil
}
