//go:build integration

package schema

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMediaSchemaSupportsPendingAssetLifecycle(t *testing.T) {
	tx := beginIntegrationTransaction(t)
	ctx := context.Background()
	assetID := uuid.New()
	trackID := uuid.New()
	uploadID := uuid.New()

	_, err := tx.Exec(ctx, `
		INSERT INTO media.media_assets (
			id, track_id, source_upload_id, original_object_key, original_filename
		) VALUES ($1, $2, $3, $4, 'demo.wav')
	`, assetID, trackID, uploadID, "original/"+assetID.String()+".wav")
	if err != nil {
		t.Fatalf("insert pending media asset with cross-service IDs: %v", err)
	}
	variantID := uuid.New()
	_, err = tx.Exec(ctx, `
		INSERT INTO media.audio_variants (
			id, media_asset_id, object_key, codec, container,
			bitrate_kbps, sample_rate, channels, size_bytes
		) VALUES ($1, $2, $3, 'aac', 'm4a', 256, 48000, 2, 1024)
	`, variantID, assetID, "variants/"+variantID.String()+".m4a")
	if err != nil {
		t.Fatalf("insert audio variant: %v", err)
	}
	jobID := uuid.New()
	_, err = tx.Exec(ctx, `
		INSERT INTO media.processing_jobs (id, media_asset_id, job_type)
		VALUES ($1, $2, 'transcode')
	`, jobID, assetID)
	if err != nil {
		t.Fatalf("insert pending processing job: %v", err)
	}

	var status string
	var duration *int
	if err = tx.QueryRow(ctx, `
		SELECT status, duration_ms FROM media.media_assets WHERE id = $1
	`, assetID).Scan(&status, &duration); err != nil {
		t.Fatalf("read media asset: %v", err)
	}
	if status != "uploaded" || duration != nil {
		t.Fatalf("asset status = %q, duration = %v", status, duration)
	}
	var startedAt, finishedAt *time.Time
	if err = tx.QueryRow(ctx, `
		SELECT started_at, finished_at FROM media.processing_jobs WHERE id = $1
	`, jobID).Scan(&startedAt, &finishedAt); err != nil {
		t.Fatalf("read processing job: %v", err)
	}
	if startedAt != nil || finishedAt != nil {
		t.Fatalf("pending job timestamps = %v, %v; want nil", startedAt, finishedAt)
	}
}

func TestMediaSchemaConstraints(t *testing.T) {
	tests := []struct {
		name string
		run  func(context.Context, pgx.Tx) error
	}{
		{
			name: "blank object key",
			run: func(ctx context.Context, tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `
					INSERT INTO media.media_assets (
						id, track_id, source_upload_id, original_object_key, original_filename
					) VALUES ($1, $2, $3, '  ', 'demo.wav')
				`, uuid.New(), uuid.New(), uuid.New())
				return err
			},
		},
		{
			name: "negative duration",
			run: func(ctx context.Context, tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `
					INSERT INTO media.media_assets (
						id, track_id, source_upload_id, original_object_key, original_filename, duration_ms
					) VALUES ($1, $2, $3, $4, 'demo.wav', -1)
				`, uuid.New(), uuid.New(), uuid.New(), "original/"+uuid.NewString())
				return err
			},
		},
		{
			name: "asset without track reference",
			run: func(ctx context.Context, tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `
					INSERT INTO media.media_assets (
						id, source_upload_id, original_object_key, original_filename
					) VALUES ($1, $2, $3, 'demo.wav')
				`, uuid.New(), uuid.New(), "original/"+uuid.NewString())
				return err
			},
		},
		{
			name: "variant references missing asset",
			run: func(ctx context.Context, tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `
					INSERT INTO media.audio_variants (
						id, media_asset_id, object_key, codec, container,
						bitrate_kbps, sample_rate, channels, size_bytes
					) VALUES ($1, $2, $3, 'aac', 'm4a', 256, 48000, 2, 1024)
				`, uuid.New(), uuid.New(), "variants/"+uuid.NewString())
				return err
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tx := beginIntegrationTransaction(t)
			if err := test.run(context.Background(), tx); err == nil {
				t.Fatal("constraint accepted invalid data")
			}
		})
	}
}

func beginIntegrationTransaction(t *testing.T) pgx.Tx {
	t.Helper()
	pool := openIntegrationDatabase(t)
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(ctx)
	})
	return tx
}

func openIntegrationDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if os.Getenv("INFINITUNE_INTEGRATION_TESTS") != "1" {
		t.Skip("set INFINITUNE_INTEGRATION_TESTS=1 to run integration tests")
	}
	port, err := strconv.ParseUint(envOrDefault("POSTGRES_PORT", "5432"), 10, 16)
	if err != nil {
		t.Fatalf("parse POSTGRES_PORT: %v", err)
	}
	config, err := pgxpool.ParseConfig("sslmode=disable")
	if err != nil {
		t.Fatalf("parse postgres config: %v", err)
	}
	config.ConnConfig.Host = envOrDefault("POSTGRES_HOST", "127.0.0.1")
	config.ConnConfig.Port = uint16(port)
	config.ConnConfig.User = requiredEnv(t, "POSTGRES_USER")
	config.ConnConfig.Password = requiredEnv(t, "POSTGRES_PASSWORD")
	config.ConnConfig.Database = requiredEnv(t, "POSTGRES_MEDIA_DB")
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

func requiredEnv(t *testing.T, key string) string {
	t.Helper()
	value := os.Getenv(key)
	if value == "" {
		t.Fatalf("%s is required", key)
	}
	return value
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
