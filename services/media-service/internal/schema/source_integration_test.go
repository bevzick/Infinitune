//go:build integration

package schema

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestMediaSchemaStoresValidatedAssetProvenance(t *testing.T) {
	tx := beginIntegrationTransaction(t)
	ctx := context.Background()
	assetID := uuid.New()
	_, err := tx.Exec(ctx, `
		INSERT INTO media.media_assets (
			id, track_id, source_upload_id, original_object_key, original_filename
		) VALUES ($1, $2, $3, $4, 'source.flac')
	`, assetID, uuid.New(), uuid.New(), "original/"+assetID.String()+".flac")
	if err != nil {
		t.Fatalf("insert media asset: %v", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO media.asset_sources (
			media_asset_id, source_kind, provider, external_id, source_url,
			license_name, license_url, attribution, allows_streaming,
			allows_redistribution, metadata
		) VALUES (
			$1, 'external_provider', 'jamendo', '12345',
			'https://www.jamendo.com/track/12345', 'CC BY 3.0',
			'https://creativecommons.org/licenses/by/3.0/', 'Example Artist',
			true, true, '{"artist":"Example Artist"}'
		)
	`, assetID)
	if err != nil {
		t.Fatalf("insert external asset source: %v", err)
	}
	var provider, artist string
	if err = tx.QueryRow(ctx, `
		SELECT provider, metadata->>'artist'
		FROM media.asset_sources WHERE media_asset_id=$1
	`, assetID).Scan(&provider, &artist); err != nil {
		t.Fatalf("read external asset source: %v", err)
	}
	if provider != "jamendo" || artist != "Example Artist" {
		t.Fatalf("stored provider = %q, artist = %q", provider, artist)
	}
}

func TestMediaSchemaRejectsInconsistentAssetProvenance(t *testing.T) {
	tx := beginIntegrationTransaction(t)
	ctx := context.Background()
	assetID := uuid.New()
	_, err := tx.Exec(ctx, `
		INSERT INTO media.media_assets (
			id, track_id, source_upload_id, original_object_key, original_filename
		) VALUES ($1, $2, $3, $4, 'source.wav')
	`, assetID, uuid.New(), uuid.New(), "original/"+assetID.String()+".wav")
	if err != nil {
		t.Fatalf("insert media asset: %v", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO media.asset_sources (
			media_asset_id, source_kind, provider, external_id, source_url,
			license_name, license_url, allows_streaming, allows_redistribution
		) VALUES (
			$1, 'local_device', 'jamendo', '12345',
			'https://www.jamendo.com/track/12345', 'CC BY',
			'https://creativecommons.org/licenses/by/3.0/', true, true
		)
	`, assetID)
	if err == nil {
		t.Fatal("schema accepted provider metadata for a local_device source")
	}
}
