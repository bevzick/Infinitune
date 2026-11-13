//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/bevzick/Infinitune/services/media-service/internal/source"
	"github.com/google/uuid"
)

func TestSourceRepositoryCreatesImmutableExternalProvenance(t *testing.T) {
	pool := openIntegrationDatabase(t)
	ctx := context.Background()
	assetID := uuid.New()
	t.Cleanup(func() { removeMediaAsset(t, pool, assetID) })
	_, err := pool.Exec(ctx, `
		INSERT INTO media.media_assets (
			id, track_id, source_upload_id, original_object_key, original_filename
		) VALUES ($1, $2, $3, $4, 'source.flac')
	`, assetID, uuid.New(), uuid.New(), "original/"+assetID.String()+".flac")
	if err != nil {
		t.Fatalf("insert media asset: %v", err)
	}
	repository := NewSourceRepository(pool)
	input := source.Record{
		MediaAssetID: assetID,
		Reference: source.Reference{
			Kind: source.KindExternalProvider, Provider: source.ProviderJamendo,
			ExternalID: "12345", SourceURL: "https://www.jamendo.com/track/12345",
			License: source.License{
				Name: "CC BY 3.0", URL: "https://creativecommons.org/licenses/by/3.0/",
				Attribution: "Example Artist", AllowsStreaming: true,
				AllowsRedistribution: true,
			},
			Metadata: map[string]string{"artist": "Example Artist"},
		},
	}
	created, inserted, err := repository.Create(ctx, input)
	if err != nil || !inserted {
		t.Fatalf("Create() = %+v, inserted %v, error %v", created, inserted, err)
	}
	if created.MediaAssetID != assetID || created.Provider != source.ProviderJamendo ||
		created.Metadata["artist"] != "Example Artist" ||
		!created.License.AllowsStreaming || !created.License.AllowsRedistribution {
		t.Fatalf("Create() = %+v", created)
	}
	existing, inserted, err := repository.Create(ctx, input)
	if err != nil || inserted || existing.CreatedAt != created.CreatedAt {
		t.Fatalf("Create(existing) = %+v, inserted %v, error %v", existing, inserted, err)
	}
	conflicting := input
	conflicting.ExternalID = "different"
	_, _, err = repository.Create(ctx, conflicting)
	if !errors.Is(err, source.ErrConflict) {
		t.Fatalf("Create(conflicting) error = %v, want %v", err, source.ErrConflict)
	}
	loaded, err := repository.ByMediaAsset(ctx, assetID)
	if err != nil || loaded.ExternalID != input.ExternalID {
		t.Fatalf("ByMediaAsset() = %+v, error %v", loaded, err)
	}
}
