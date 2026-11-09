//go:build integration

package postgres

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/media-service/internal/media"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMediaRepositoryReturnsAssetWithOrderedVariants(t *testing.T) {
	pool := openIntegrationDatabase(t)
	ctx := context.Background()
	assetID := uuid.New()
	trackID := uuid.New()
	defer removeMediaAsset(t, pool, assetID)

	_, err := pool.Exec(ctx, `
		INSERT INTO media.media_assets (
			id, track_id, source_upload_id, original_object_key, original_filename,
			duration_ms, status
		) VALUES ($1, $2, $3, $4, $5, $6, 'ready')
	`, assetID, trackID, uuid.New(), "original/"+assetID.String(), "track.wav", 180000)
	if err != nil {
		t.Fatalf("insert media asset: %v", err)
	}
	for _, bitrate := range []int{320, 128} {
		_, err = pool.Exec(ctx, `
			INSERT INTO media.audio_variants (
				id, media_asset_id, object_key, codec, container, bitrate_kbps,
				sample_rate, channels, size_bytes
			) VALUES ($1, $2, $3, 'aac', 'm4a', $4, 48000, 2, 4096)
		`, uuid.New(), assetID, "variants/"+assetID.String()+"/"+strconv.Itoa(bitrate), bitrate)
		if err != nil {
			t.Fatalf("insert audio variant: %v", err)
		}
	}

	asset, err := NewMediaRepository(pool).AssetByID(ctx, assetID)
	if err != nil {
		t.Fatalf("AssetByID() error = %v", err)
	}
	if asset.TrackID != trackID || asset.Status != media.StatusReady || asset.DurationMS == nil ||
		*asset.DurationMS != 180000 || len(asset.Variants) != 2 ||
		asset.Variants[0].BitrateKbps != 128 || asset.Variants[1].BitrateKbps != 320 {
		t.Fatalf("AssetByID() = %+v", asset)
	}

	_, err = NewMediaRepository(pool).AssetByID(ctx, uuid.New())
	if !errors.Is(err, media.ErrNotFound) {
		t.Fatalf("AssetByID(missing) error = %v, want %v", err, media.ErrNotFound)
	}
}

func TestMediaRepositoryCreatesCreatorAssetIdempotently(t *testing.T) {
	pool := openIntegrationDatabase(t)
	ctx := context.Background()
	assetID, contentTrackID, uploadID := uuid.New(), uuid.New(), uuid.New()
	t.Cleanup(func() { removeMediaAsset(t, pool, assetID) })
	repository := NewMediaRepository(pool)
	input := media.Asset{
		ID: assetID, ContentTrackID: contentTrackID, SourceUploadID: uploadID,
		OriginalObjectKey: "creator-content/" + uploadID.String() + "/source.flac",
		OriginalFilename:  "source.flac", Status: media.StatusUploaded,
	}
	created, inserted, err := repository.CreateFromUpload(ctx, input)
	if err != nil || !inserted {
		t.Fatalf("CreateFromUpload() = %+v, inserted %v, %v", created, inserted, err)
	}
	if created.TrackID != uuid.Nil || created.ContentTrackID != contentTrackID ||
		created.SourceUploadID != uploadID || created.Status != media.StatusUploaded {
		t.Fatalf("CreateFromUpload() = %+v", created)
	}
	duplicate := input
	duplicate.ID = uuid.New()
	idempotent, inserted, err := repository.CreateFromUpload(ctx, duplicate)
	if err != nil || inserted || idempotent.ID != assetID {
		t.Fatalf("CreateFromUpload(idempotent) = %+v, inserted %v, %v",
			idempotent, inserted, err)
	}
	loaded, err := repository.AssetBySourceUpload(ctx, uploadID)
	if err != nil || loaded.ID != assetID || loaded.ContentTrackID != contentTrackID {
		t.Fatalf("AssetBySourceUpload() = %+v, %v", loaded, err)
	}
}

func openIntegrationDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if os.Getenv("INFINITUNE_INTEGRATION_TESTS") != "1" {
		t.Skip("set INFINITUNE_INTEGRATION_TESTS=1 to run integration tests")
	}
	port, err := strconv.ParseUint(environmentOrDefault("POSTGRES_PORT", "5432"), 10, 16)
	if err != nil {
		t.Fatalf("parse POSTGRES_PORT: %v", err)
	}
	config, err := pgxpool.ParseConfig("sslmode=disable")
	if err != nil {
		t.Fatalf("parse postgres config: %v", err)
	}
	config.ConnConfig.Host = environmentOrDefault("POSTGRES_HOST", "127.0.0.1")
	config.ConnConfig.Port = uint16(port)
	config.ConnConfig.User = requiredEnvironment(t, "POSTGRES_USER")
	config.ConnConfig.Password = requiredEnvironment(t, "POSTGRES_PASSWORD")
	config.ConnConfig.Database = requiredEnvironment(t, "POSTGRES_MEDIA_DB")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping postgres: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func requiredEnvironment(t *testing.T, key string) string {
	t.Helper()
	value := os.Getenv(key)
	if value == "" {
		t.Fatalf("%s is required", key)
	}
	return value
}

func environmentOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func removeMediaAsset(t *testing.T, pool *pgxpool.Pool, assetID uuid.UUID) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`DELETE FROM media.audio_variants WHERE media_asset_id = $1`, assetID); err != nil {
		t.Errorf("delete audio variants: %v", err)
	}
	if _, err := pool.Exec(context.Background(),
		`DELETE FROM media.media_assets WHERE id = $1`, assetID); err != nil {
		t.Errorf("delete media asset: %v", err)
	}
}
