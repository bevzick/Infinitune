package media

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestProjectedStoreUsesProjection(t *testing.T) {
	want := Asset{ID: uuid.New()}
	projection := &assetStoreStub{asset: want}
	legacy := &assetStoreStub{}
	got, err := NewProjectedStore(projection, legacy).AssetByUpload(context.Background(), uuid.New())
	if err != nil || got.ID != want.ID || legacy.calls != 0 {
		t.Fatalf("AssetByUpload() = %+v, %v; legacy calls = %d", got, err, legacy.calls)
	}
}

func TestProjectedStoreFallsBackForLegacyAsset(t *testing.T) {
	want := Asset{ID: uuid.New()}
	projection := &assetStoreStub{err: ErrNotFound}
	legacy := &assetStoreStub{asset: want}
	got, err := NewProjectedStore(projection, legacy).AssetByUpload(context.Background(), uuid.New())
	if err != nil || got.ID != want.ID || legacy.calls != 1 {
		t.Fatalf("AssetByUpload() = %+v, %v; legacy calls = %d", got, err, legacy.calls)
	}
}

func TestProjectedStoreDoesNotMaskProjectionFailure(t *testing.T) {
	projection := &assetStoreStub{err: errors.New("database unavailable")}
	legacy := &assetStoreStub{}
	_, err := NewProjectedStore(projection, legacy).AssetByUpload(context.Background(), uuid.New())
	if err == nil || legacy.calls != 0 {
		t.Fatalf("AssetByUpload() error = %v; legacy calls = %d", err, legacy.calls)
	}
}

func TestProjectedStoreLoadsExternalAssetByTrack(t *testing.T) {
	want := Asset{ID: uuid.New(), ContentTrackID: uuid.New()}
	projection := &assetStoreStub{asset: want}
	got, err := NewProjectedStore(projection, &assetStoreStub{}).
		AssetByTrack(context.Background(), want.ContentTrackID)
	if err != nil || got.ID != want.ID || projection.trackCalls != 1 {
		t.Fatalf("AssetByTrack() = %+v, %v; calls = %d", got, err, projection.trackCalls)
	}
}

type assetStoreStub struct {
	asset      Asset
	err        error
	calls      int
	trackCalls int
}

func (stub *assetStoreStub) AssetByTrack(context.Context, uuid.UUID) (Asset, error) {
	stub.trackCalls++
	return stub.asset, stub.err
}

func (stub *assetStoreStub) AssetByUpload(context.Context, uuid.UUID) (Asset, error) {
	stub.calls++
	return stub.asset, stub.err
}
