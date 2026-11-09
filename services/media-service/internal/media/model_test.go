package media

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type storeStub struct {
	asset Asset
	err   error
	calls int
}

func (stub *storeStub) AssetBySourceUpload(context.Context, uuid.UUID) (Asset, error) {
	stub.calls++
	return stub.asset, stub.err
}

func (stub *storeStub) AssetByID(context.Context, uuid.UUID) (Asset, error) {
	stub.calls++
	return stub.asset, stub.err
}

func TestServiceRejectsNilAssetID(t *testing.T) {
	store := &storeStub{}
	_, err := NewService(store).Asset(context.Background(), uuid.Nil)
	if !errors.Is(err, ErrInvalid) || store.calls != 0 {
		t.Fatalf("Asset() error = %v, calls = %d", err, store.calls)
	}
}

func TestServiceLoadsAssetByUpload(t *testing.T) {
	uploadID := uuid.New()
	store := &storeStub{asset: Asset{ID: uuid.New(), SourceUploadID: uploadID}}
	asset, err := NewService(store).AssetByUpload(context.Background(), uploadID)
	if err != nil || asset.SourceUploadID != uploadID || store.calls != 1 {
		t.Fatalf("AssetByUpload() = %+v, %v, calls %d", asset, err, store.calls)
	}
	_, err = NewService(store).AssetByUpload(context.Background(), uuid.Nil)
	if !errors.Is(err, ErrInvalid) || store.calls != 1 {
		t.Fatalf("AssetByUpload(nil) error = %v, calls %d", err, store.calls)
	}
}

func TestServiceReturnsOnlyReadyAssetForPlayback(t *testing.T) {
	id := uuid.New()
	ready := &storeStub{asset: Asset{ID: id, Status: StatusReady}}
	asset, err := NewService(ready).ReadyAsset(context.Background(), id)
	if err != nil || asset.ID != id {
		t.Fatalf("ReadyAsset() = %+v, %v", asset, err)
	}

	notReady := &storeStub{asset: Asset{ID: id, Status: StatusProcessing}}
	_, err = NewService(notReady).ReadyAsset(context.Background(), id)
	if !errors.Is(err, ErrNotReady) {
		t.Fatalf("ReadyAsset() error = %v, want %v", err, ErrNotReady)
	}
}

func TestServicePreservesMissingAssetError(t *testing.T) {
	store := &storeStub{err: ErrNotFound}
	_, err := NewService(store).ReadyAsset(context.Background(), uuid.New())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("ReadyAsset() error = %v, want %v", err, ErrNotFound)
	}
}

func TestServiceSelectsHighestBitratePlaybackVariant(t *testing.T) {
	id := uuid.New()
	store := &storeStub{asset: Asset{
		ID: id, Status: StatusReady,
		Variants: []AudioVariant{
			{ID: uuid.New(), BitrateKbps: 128},
			{ID: uuid.New(), BitrateKbps: 320},
			{ID: uuid.New(), BitrateKbps: 256},
		},
	}}
	_, variant, err := NewService(store).PlaybackVariant(context.Background(), id)
	if err != nil || variant.BitrateKbps != 320 {
		t.Fatalf("PlaybackVariant() = %+v, %v", variant, err)
	}
}

func TestServiceRejectsReadyAssetWithoutVariant(t *testing.T) {
	store := &storeStub{asset: Asset{ID: uuid.New(), Status: StatusReady}}
	_, _, err := NewService(store).PlaybackVariant(context.Background(), store.asset.ID)
	if !errors.Is(err, ErrNotReady) {
		t.Fatalf("PlaybackVariant() error = %v, want %v", err, ErrNotReady)
	}
}
