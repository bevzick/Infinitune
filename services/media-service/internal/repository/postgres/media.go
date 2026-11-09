package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bevzick/Infinitune/services/media-service/internal/media"
	"github.com/bevzick/Infinitune/services/media-service/internal/processing"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type mediaDatabase interface {
	Begin(context.Context) (pgx.Tx, error)
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type MediaRepository struct {
	database mediaDatabase
}

var _ media.Store = (*MediaRepository)(nil)
var _ processing.Queue = (*MediaRepository)(nil)

func NewMediaRepository(database mediaDatabase) *MediaRepository {
	return &MediaRepository{database: database}
}

func (repository *MediaRepository) AssetByID(
	ctx context.Context,
	id uuid.UUID,
) (media.Asset, error) {
	var asset media.Asset
	var trackID, contentTrackID *uuid.UUID
	err := repository.database.QueryRow(ctx, `
		SELECT
			id, track_id, content_track_id, source_upload_id, original_object_key, original_filename,
			duration_ms, status, created_at, updated_at
		FROM media.media_assets
		WHERE id = $1
	`, id).Scan(
		&asset.ID,
		&trackID,
		&contentTrackID,
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
	if trackID != nil {
		asset.TrackID = *trackID
	}
	if contentTrackID != nil {
		asset.ContentTrackID = *contentTrackID
	}

	asset.Variants, err = repository.audioVariants(ctx, id)
	if err != nil {
		return media.Asset{}, fmt.Errorf("query audio variants: %w", err)
	}
	return asset, nil
}

func (repository *MediaRepository) CreateFromUpload(
	ctx context.Context,
	asset media.Asset,
) (media.Asset, bool, error) {
	created, err := scanAsset(repository.database.QueryRow(ctx, `
		INSERT INTO media.media_assets (
			id, track_id, content_track_id, source_upload_id,
			original_object_key, original_filename, status
		)
		VALUES ($1, NULL, $2, $3, $4, $5, $6)
		ON CONFLICT (source_upload_id) DO NOTHING
		RETURNING id, track_id, content_track_id, source_upload_id,
		          original_object_key, original_filename, duration_ms,
		          status, created_at, updated_at
	`, asset.ID, asset.ContentTrackID, asset.SourceUploadID,
		asset.OriginalObjectKey, asset.OriginalFilename, asset.Status))
	if err == nil {
		return created, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return media.Asset{}, false, fmt.Errorf("insert media asset from upload: %w", err)
	}
	existing, err := repository.AssetBySourceUpload(ctx, asset.SourceUploadID)
	if err != nil {
		return media.Asset{}, false, fmt.Errorf("find idempotent media asset: %w", err)
	}
	return existing, false, nil
}

func (repository *MediaRepository) EnqueueProcessing(
	ctx context.Context,
	assetID uuid.UUID,
) error {
	tx, err := repository.database.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin enqueue processing: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `
		INSERT INTO media.processing_jobs (
			id, media_asset_id, job_type, status, available_at
		)
		VALUES ($1, $2, $3, 'pending', now())
		ON CONFLICT (media_asset_id, job_type) DO UPDATE
		SET status = 'pending', attempts = 0, error_message = NULL,
		    started_at = NULL, finished_at = NULL, available_at = now()
		WHERE media.processing_jobs.status = 'failed'
	`, uuid.New(), assetID, processing.JobTypeAudioVariants)
	if err != nil {
		return fmt.Errorf("enqueue audio processing: %w", err)
	}
	_, err = tx.Exec(ctx, `
		UPDATE media.media_assets AS asset
		SET status = 'uploaded', updated_at = now()
		WHERE asset.id = $1
		  AND asset.status = 'failed'
		  AND EXISTS (
		      SELECT 1 FROM media.processing_jobs AS job
		      WHERE job.media_asset_id = asset.id
		        AND job.job_type = $2 AND job.status = 'pending'
		  )
	`, assetID, processing.JobTypeAudioVariants)
	if err != nil {
		return fmt.Errorf("reset media asset for processing: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit enqueue processing: %w", err)
	}
	return nil
}

func (repository *MediaRepository) ClaimProcessing(
	ctx context.Context,
) (processing.Job, error) {
	tx, err := repository.database.Begin(ctx)
	if err != nil {
		return processing.Job{}, fmt.Errorf("begin claim processing: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var job processing.Job
	err = tx.QueryRow(ctx, `
		WITH candidate AS (
			SELECT id
			FROM media.processing_jobs
			WHERE status = 'pending' AND available_at <= now()
			ORDER BY available_at, created_at, id
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		UPDATE media.processing_jobs AS job
		SET status = 'running', attempts = job.attempts + 1,
		    error_message = NULL, started_at = now(), finished_at = NULL
		FROM candidate
		WHERE job.id = candidate.id
		RETURNING job.id, job.media_asset_id, job.attempts
	`).Scan(&job.ID, &job.Asset.ID, &job.Attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return processing.Job{}, processing.ErrNoPendingJob
	}
	if err != nil {
		return processing.Job{}, fmt.Errorf("claim audio processing: %w", err)
	}
	job.Asset, err = scanAsset(tx.QueryRow(ctx, `
		UPDATE media.media_assets
		SET status = 'processing', updated_at = now()
		WHERE id = $1
		RETURNING id, track_id, content_track_id, source_upload_id,
		          original_object_key, original_filename, duration_ms,
		          status, created_at, updated_at
	`, job.Asset.ID))
	if err != nil {
		return processing.Job{}, fmt.Errorf("mark media asset processing: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return processing.Job{}, fmt.Errorf("commit claim processing: %w", err)
	}
	return job, nil
}

func (repository *MediaRepository) CompleteProcessing(
	ctx context.Context,
	job processing.Job,
	durationMS int,
	variants []media.AudioVariant,
) error {
	tx, err := repository.database.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin complete processing: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, variant := range variants {
		_, err = tx.Exec(ctx, `
			INSERT INTO media.audio_variants (
				id, media_asset_id, object_key, codec, container, bitrate_kbps,
				sample_rate, channels, size_bytes
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (media_asset_id, codec, container, bitrate_kbps) DO UPDATE
			SET object_key = EXCLUDED.object_key,
			    sample_rate = EXCLUDED.sample_rate,
			    channels = EXCLUDED.channels,
			    size_bytes = EXCLUDED.size_bytes,
			    created_at = now()
		`, variant.ID, job.Asset.ID, variant.ObjectKey, variant.Codec,
			variant.Container, variant.BitrateKbps, variant.SampleRate,
			variant.Channels, variant.SizeBytes)
		if err != nil {
			return fmt.Errorf("store audio variant metadata: %w", err)
		}
	}
	commandTag, err := tx.Exec(ctx, `
		UPDATE media.processing_jobs
		SET status = 'completed', error_message = NULL, finished_at = now()
		WHERE id = $1 AND media_asset_id = $2 AND status = 'running'
	`, job.ID, job.Asset.ID)
	if err != nil {
		return fmt.Errorf("complete audio processing job: %w", err)
	}
	if commandTag.RowsAffected() != 1 {
		return fmt.Errorf("processing job is not running")
	}
	_, err = tx.Exec(ctx, `
		UPDATE media.media_assets
		SET status = 'ready', duration_ms = $2, updated_at = now()
		WHERE id = $1
	`, job.Asset.ID, durationMS)
	if err != nil {
		return fmt.Errorf("mark media asset ready: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit completed processing: %w", err)
	}
	return nil
}

func (repository *MediaRepository) FailProcessing(
	ctx context.Context,
	job processing.Job,
	errorMessage string,
	retryAt time.Time,
	maxAttempts int,
) (bool, error) {
	terminal := job.Attempts >= maxAttempts
	status := "pending"
	assetStatus := media.StatusUploaded
	if terminal {
		status = "failed"
		assetStatus = media.StatusFailed
	}
	tx, err := repository.database.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin fail processing: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	commandTag, err := tx.Exec(ctx, `
		UPDATE media.processing_jobs
		SET status = $3::media.processing_status, error_message = $4, available_at = $5,
		    started_at = CASE WHEN $3::text = 'pending' THEN NULL ELSE started_at END,
		    finished_at = CASE WHEN $3::text = 'failed' THEN now() ELSE NULL END
		WHERE id = $1 AND media_asset_id = $2 AND status = 'running'
	`, job.ID, job.Asset.ID, status, errorMessage, retryAt)
	if err != nil {
		return false, fmt.Errorf("record audio processing failure: %w", err)
	}
	if commandTag.RowsAffected() != 1 {
		return false, fmt.Errorf("processing job is not running")
	}
	_, err = tx.Exec(ctx, `
		UPDATE media.media_assets SET status = $2, updated_at = now() WHERE id = $1
	`, job.Asset.ID, assetStatus)
	if err != nil {
		return false, fmt.Errorf("update failed media asset: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit processing failure: %w", err)
	}
	return terminal, nil
}

func (repository *MediaRepository) RecoverStaleProcessing(
	ctx context.Context,
	staleBefore time.Time,
) (int64, error) {
	commandTag, err := repository.database.Exec(ctx, `
		WITH recovered AS (
			UPDATE media.processing_jobs
			SET status = 'pending', error_message = 'worker interrupted',
			    started_at = NULL, finished_at = NULL, available_at = now()
			WHERE status = 'running' AND started_at < $1
			RETURNING media_asset_id
		)
		UPDATE media.media_assets AS asset
		SET status = 'uploaded', updated_at = now()
		WHERE asset.id IN (SELECT media_asset_id FROM recovered)
	`, staleBefore)
	if err != nil {
		return 0, fmt.Errorf("recover stale processing jobs: %w", err)
	}
	return commandTag.RowsAffected(), nil
}

func (repository *MediaRepository) AssetBySourceUpload(
	ctx context.Context,
	uploadID uuid.UUID,
) (media.Asset, error) {
	asset, err := scanAsset(repository.database.QueryRow(ctx, `
		SELECT id, track_id, content_track_id, source_upload_id,
		       original_object_key, original_filename, duration_ms,
		       status, created_at, updated_at
		FROM media.media_assets
		WHERE source_upload_id = $1
	`, uploadID))
	if errors.Is(err, pgx.ErrNoRows) {
		return media.Asset{}, media.ErrNotFound
	}
	if err != nil {
		return media.Asset{}, fmt.Errorf("query media asset by source upload: %w", err)
	}
	asset.Variants = make([]media.AudioVariant, 0)
	return asset, nil
}

func scanAsset(row pgx.Row) (media.Asset, error) {
	var asset media.Asset
	var trackID, contentTrackID *uuid.UUID
	err := row.Scan(
		&asset.ID, &trackID, &contentTrackID, &asset.SourceUploadID,
		&asset.OriginalObjectKey, &asset.OriginalFilename, &asset.DurationMS,
		&asset.Status, &asset.CreatedAt, &asset.UpdatedAt,
	)
	if trackID != nil {
		asset.TrackID = *trackID
	}
	if contentTrackID != nil {
		asset.ContentTrackID = *contentTrackID
	}
	return asset, err
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
