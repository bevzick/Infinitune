//go:build integration

package devseed

import (
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/media-service/internal/config"
	"github.com/bevzick/Infinitune/services/media-service/internal/database"
	"github.com/bevzick/Infinitune/services/media-service/internal/storage"
	"github.com/google/uuid"
)

func TestDevelopmentSeedIsIdempotentAndPlayable(t *testing.T) {
	if os.Getenv("INFINITUNE_INTEGRATION_TESTS") != "1" {
		t.Skip("set INFINITUNE_INTEGRATION_TESTS=1 to run integration tests")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	databasePool, err := database.Open(ctx, cfg.Database)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer databasePool.Close()
	objectStorage, err := storage.Open(cfg.Storage)
	if err != nil {
		t.Fatalf("open object storage: %v", err)
	}

	if err = Seed(ctx, databasePool, objectStorage); err != nil {
		t.Fatalf("Seed(first) error = %v", err)
	}
	if err = Seed(ctx, databasePool, objectStorage); err != nil {
		t.Fatalf("Seed(second) error = %v", err)
	}

	assetIDs := make([]uuid.UUID, 0, len(developmentAssets))
	for _, asset := range developmentAssets {
		assetIDs = append(assetIDs, asset.assetID)
	}
	var assetCount int
	err = databasePool.QueryRow(ctx, `
		SELECT count(*)
		FROM media.media_assets
		WHERE id = ANY($1) AND status = 'ready' AND duration_ms = $2
	`, assetIDs, demoDurationMS).Scan(&assetCount)
	if err != nil || assetCount != len(developmentAssets) {
		t.Fatalf("ready asset count = %d, %v; want %d", assetCount, err, len(developmentAssets))
	}
	var variantCount int
	err = databasePool.QueryRow(ctx, `
		SELECT count(*)
		FROM media.audio_variants
		WHERE media_asset_id = ANY($1)
	`, assetIDs).Scan(&variantCount)
	if err != nil || variantCount != len(developmentAssets) {
		t.Fatalf("variant count = %d, %v; want %d", variantCount, err, len(developmentAssets))
	}

	object, err := objectStorage.Open(ctx, developmentAssets[0].objectKey)
	if err != nil {
		t.Fatalf("open seeded object: %v", err)
	}
	defer object.Body.Close()
	header := make([]byte, wavHeaderSize)
	if _, err = io.ReadFull(object.Body, header); err != nil {
		t.Fatalf("read seeded WAV header: %v", err)
	}
	wantSize := int64(wavHeaderSize + sampleRate*demoDurationMS/1000*bitsPerSample/8)
	if string(header[0:4]) != "RIFF" || string(header[8:12]) != "WAVE" ||
		object.ContentType != "audio/wav" || object.Size != wantSize {
		t.Fatalf("seeded object = size %d, content-type %q, header %q/%q",
			object.Size, object.ContentType, header[0:4], header[8:12])
	}
}
