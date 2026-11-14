package devseed

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Database interface {
	Begin(context.Context) (pgx.Tx, error)
}

type ObjectStorage interface {
	Put(context.Context, string, io.Reader, int64, string) error
}

type assetSeed struct {
	assetID   uuid.UUID
	trackID   uuid.UUID
	uploadID  uuid.UUID
	variantID uuid.UUID
	filename  string
	objectKey string
	melody    []float64
}

var developmentAssets = []assetSeed{
	newAssetSeed(1, "first-light-engine-demo.wav", []float64{261.63, 329.63, 392.00, 523.25}),
	newAssetSeed(2, "paper-moons-demo.wav", []float64{220.00, 277.18, 329.63, 440.00}),
	newAssetSeed(3, "harbor-without-maps-demo.wav", []float64{196.00, 246.94, 293.66, 392.00}),
	newAssetSeed(4, "neon-rain-index-demo.wav", []float64{293.66, 369.99, 440.00, 587.33}),
	newAssetSeed(5, "parallel-sunrise-demo.wav", []float64{246.94, 311.13, 369.99, 493.88}),
	newAssetSeed(6, "cedar-signal-demo.wav", []float64{174.61, 220.00, 261.63, 349.23}),
}

func newAssetSeed(index int, filename string, melody []float64) assetSeed {
	suffix := fmt.Sprintf("%012d", index)
	assetID := uuid.MustParse("40000000-0000-4000-8000-" + suffix)
	return assetSeed{
		assetID:   assetID,
		trackID:   uuid.MustParse("30000000-0000-4000-8000-" + suffix),
		uploadID:  uuid.MustParse("80000000-0000-4000-8000-" + suffix),
		variantID: uuid.MustParse("90000000-0000-4000-8000-" + suffix),
		filename:  filename,
		objectKey: "demo/" + assetID.String() + "/playback.wav",
		melody:    melody,
	}
}

func Seed(ctx context.Context, database Database, objects ObjectStorage) error {
	payloads := make([][]byte, len(developmentAssets))
	for index, asset := range developmentAssets {
		payloads[index] = generateWAV(asset.melody)
		if err := objects.Put(ctx, asset.objectKey, bytes.NewReader(payloads[index]),
			int64(len(payloads[index])), "audio/wav"); err != nil {
			return fmt.Errorf("store demo asset %s: %w", asset.assetID, err)
		}
	}

	transaction, err := database.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin demo seed transaction: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	for index, asset := range developmentAssets {
		if _, err = transaction.Exec(ctx, `
			INSERT INTO media.media_assets (
				id, track_id, source_upload_id, original_object_key, original_filename,
				duration_ms, status
			) VALUES ($1, $2, $3, $4, $5, $6, 'ready')
			ON CONFLICT (id) DO UPDATE SET
				track_id = EXCLUDED.track_id,
				source_upload_id = EXCLUDED.source_upload_id,
				original_object_key = EXCLUDED.original_object_key,
				original_filename = EXCLUDED.original_filename,
				duration_ms = EXCLUDED.duration_ms,
				status = EXCLUDED.status,
				updated_at = now()
		`, asset.assetID, asset.trackID, asset.uploadID, asset.objectKey,
			asset.filename, demoDurationMS); err != nil {
			return fmt.Errorf("upsert demo media asset %s: %w", asset.assetID, err)
		}
		if _, err = transaction.Exec(ctx, `
			INSERT INTO media.audio_variants (
				id, media_asset_id, object_key, codec, container, bitrate_kbps,
				sample_rate, channels, size_bytes
			) VALUES ($1, $2, $3, 'pcm_s16le', 'wav', $4, $5, $6, $7)
			ON CONFLICT (id) DO UPDATE SET
				media_asset_id = EXCLUDED.media_asset_id,
				object_key = EXCLUDED.object_key,
				codec = EXCLUDED.codec,
				container = EXCLUDED.container,
				bitrate_kbps = EXCLUDED.bitrate_kbps,
				sample_rate = EXCLUDED.sample_rate,
				channels = EXCLUDED.channels,
				size_bytes = EXCLUDED.size_bytes
		`, asset.variantID, asset.assetID, asset.objectKey,
			sampleRate*bitsPerSample*channelCount/1000, sampleRate, channelCount,
			len(payloads[index])); err != nil {
			return fmt.Errorf("upsert demo audio variant %s: %w", asset.variantID, err)
		}
	}
	if err = transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit demo seed transaction: %w", err)
	}
	return nil
}
