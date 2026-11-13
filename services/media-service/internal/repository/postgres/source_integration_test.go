//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/bevzick/Infinitune/services/media-service/internal/media"
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

func TestMediaRepositoryCreatesExternalAssetSourceAndJobAtomically(t *testing.T) {
	pool := openIntegrationDatabase(t)
	ctx := context.Background()
	assetID, trackID := uuid.New(), uuid.New()
	t.Cleanup(func() { removeMediaAsset(t, pool, assetID) })
	reference := source.Reference{
		Kind: source.KindExternalProvider, Provider: source.ProviderJamendo,
		ExternalID: "42", SourceURL: "https://www.jamendo.com/track/42",
		License: source.License{
			Name: "CC BY 4.0", URL: "https://creativecommons.org/licenses/by/4.0/",
			Attribution: "Luna Veil", AllowsStreaming: true, AllowsRedistribution: true,
		},
		Metadata: map[string]string{"title": "Night Signal"},
	}
	repository := NewMediaRepository(pool)
	asset, created, err := repository.CreateExternal(ctx, media.Asset{
		ID: assetID, ContentTrackID: trackID,
		OriginalObjectKey: "external-imports/" + assetID.String() + "/track.mp3",
		OriginalFilename:  "track.mp3", Status: media.StatusUploaded,
	}, reference)
	if err != nil || !created || asset.SourceUploadID != uuid.Nil {
		t.Fatalf("CreateExternal() = %+v, created %v, error %v", asset, created, err)
	}
	loaded, loadedReference, err := repository.ExternalByContentTrack(ctx, trackID)
	if err != nil || loaded.ID != assetID || loadedReference.ExternalID != "42" {
		t.Fatalf("ExternalByContentTrack() = %+v, %+v, %v", loaded, loadedReference, err)
	}
	var jobs int
	if err = pool.QueryRow(ctx,
		`SELECT count(*) FROM media.processing_jobs WHERE media_asset_id = $1`, assetID,
	).Scan(&jobs); err != nil || jobs != 1 {
		t.Fatalf("processing jobs = %d, error %v", jobs, err)
	}
}
