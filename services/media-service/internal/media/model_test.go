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

type authorizerStub struct {
	allowed bool
	err     error
	trackID uuid.UUID
	calls   int
}

func (stub *authorizerStub) CanStream(
	_ context.Context,
	trackID uuid.UUID,
) (bool, error) {
	stub.calls++
	stub.trackID = trackID
	return stub.allowed, stub.err
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
	_, err := NewService(store, nil).Asset(context.Background(), uuid.Nil)
	if !errors.Is(err, ErrInvalid) || store.calls != 0 {
		t.Fatalf("Asset() error = %v, calls = %d", err, store.calls)
	}
}

func TestServiceLoadsAssetByUpload(t *testing.T) {
	uploadID := uuid.New()
	store := &storeStub{asset: Asset{ID: uuid.New(), SourceUploadID: uploadID}}
	asset, err := NewService(store, nil).AssetByUpload(context.Background(), uploadID)
	if err != nil || asset.SourceUploadID != uploadID || store.calls != 1 {
		t.Fatalf("AssetByUpload() = %+v, %v, calls %d", asset, err, store.calls)
	}
	_, err = NewService(store, nil).AssetByUpload(context.Background(), uuid.Nil)
	if !errors.Is(err, ErrInvalid) || store.calls != 1 {
		t.Fatalf("AssetByUpload(nil) error = %v, calls %d", err, store.calls)
	}
}

func TestServiceReturnsOnlyReadyAssetForPlayback(t *testing.T) {
	id := uuid.New()
	ready := &storeStub{asset: Asset{ID: id, Status: StatusReady}}
	asset, err := NewService(ready, nil).ReadyAsset(context.Background(), id)
	if err != nil || asset.ID != id {
		t.Fatalf("ReadyAsset() = %+v, %v", asset, err)
	}

	notReady := &storeStub{asset: Asset{ID: id, Status: StatusProcessing}}
	_, err = NewService(notReady, nil).ReadyAsset(context.Background(), id)
	if !errors.Is(err, ErrNotReady) {
		t.Fatalf("ReadyAsset() error = %v, want %v", err, ErrNotReady)
	}
}

func TestServicePreservesMissingAssetError(t *testing.T) {
	store := &storeStub{err: ErrNotFound}
	_, err := NewService(store, nil).ReadyAsset(context.Background(), uuid.New())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("ReadyAsset() error = %v, want %v", err, ErrNotFound)
	}
}

func TestServiceSelectsHighestBitratePlaybackVariant(t *testing.T) {
	id := uuid.New()
	contentTrackID := uuid.New()
	store := &storeStub{asset: Asset{
		ID: id, ContentTrackID: contentTrackID, Status: StatusReady,
		Variants: []AudioVariant{
			{ID: uuid.New(), BitrateKbps: 128},
			{ID: uuid.New(), BitrateKbps: 320},
			{ID: uuid.New(), BitrateKbps: 256},
		},
	}}
	authorizer := &authorizerStub{allowed: true}
	_, variant, err := NewService(store, authorizer).PlaybackVariant(
		context.Background(), id, AudioQualityHigh,
	)
	if err != nil || variant.BitrateKbps != 320 || authorizer.trackID != contentTrackID {
		t.Fatalf("PlaybackVariant() = %+v, %v; authorizer = %+v", variant, err, authorizer)
	}
}

func TestSelectAudioVariantUsesAvailableQualityWithFallback(t *testing.T) {
	variants := []AudioVariant{
		{ID: uuid.New(), Codec: "mp3", BitrateKbps: 128},
		{ID: uuid.New(), Codec: "mp3", BitrateKbps: 320},
		{ID: uuid.New(), Codec: "flac", BitrateKbps: 900},
	}
	tests := []struct {
		quality       AudioQuality
		wantBitrate   int
		wantEffective AudioQuality
	}{
		{quality: AudioQualityLow, wantBitrate: 128, wantEffective: AudioQualityLow},
		{quality: AudioQualityMedium, wantBitrate: 128, wantEffective: AudioQualityLow},
		{quality: AudioQualityHigh, wantBitrate: 320, wantEffective: AudioQualityHigh},
		{quality: AudioQualityLossless, wantBitrate: 900, wantEffective: AudioQualityLossless},
	}
	for _, test := range tests {
		t.Run(string(test.quality), func(t *testing.T) {
			selected := selectAudioVariant(variants, test.quality)
			if selected.BitrateKbps != test.wantBitrate || QualityOf(selected) != test.wantEffective {
				t.Fatalf("selected = %+v", selected)
			}
		})
	}
	withoutLossless := selectAudioVariant(variants[:2], AudioQualityLossless)
	if withoutLossless.BitrateKbps != 320 || QualityOf(withoutLossless) != AudioQualityHigh {
		t.Fatalf("lossless fallback = %+v", withoutLossless)
	}
	lowFallback := selectAudioVariant(variants[1:2], AudioQualityLow)
	if lowFallback.BitrateKbps != 320 {
		t.Fatalf("low fallback = %+v", lowFallback)
	}
}

func TestPlaybackVariantRejectsUnknownQualityBeforeLoadingAsset(t *testing.T) {
	store := &storeStub{}
	_, _, err := NewService(store, nil).PlaybackVariant(
		context.Background(), uuid.New(), "studio",
	)
	if !errors.Is(err, ErrInvalidQuality) || store.calls != 0 {
		t.Fatalf("PlaybackVariant() error = %v; store calls = %d", err, store.calls)
	}
}

func TestServiceRejectsReadyAssetWithoutVariant(t *testing.T) {
	store := &storeStub{asset: Asset{ID: uuid.New(), TrackID: uuid.New(), Status: StatusReady}}
	_, _, err := NewService(store, nil).PlaybackVariant(
		context.Background(), store.asset.ID, AudioQualityHigh,
	)
	if !errors.Is(err, ErrNotReady) {
		t.Fatalf("PlaybackVariant() error = %v, want %v", err, ErrNotReady)
	}
}

func TestServiceHidesUnpublishedContentBeforeReadinessCheck(t *testing.T) {
	store := &storeStub{asset: Asset{
		ID: uuid.New(), ContentTrackID: uuid.New(), Status: StatusProcessing,
	}}
	_, _, err := NewService(store, &authorizerStub{allowed: false}).PlaybackVariant(
		context.Background(), store.asset.ID, AudioQualityHigh,
	)
	if !errors.Is(err, ErrNotPlayable) {
		t.Fatalf("PlaybackVariant() error = %v, want %v", err, ErrNotPlayable)
	}
}

func TestServiceHidesUnpublishedContentAsset(t *testing.T) {
	trackID := uuid.New()
	store := &storeStub{asset: Asset{
		ID: uuid.New(), ContentTrackID: trackID, Status: StatusReady,
		Variants: []AudioVariant{{ID: uuid.New(), BitrateKbps: 320}},
	}}
	authorizer := &authorizerStub{allowed: false}
	_, _, err := NewService(store, authorizer).PlaybackVariant(
		context.Background(), store.asset.ID, AudioQualityHigh,
	)
	if !errors.Is(err, ErrNotPlayable) || authorizer.calls != 1 || authorizer.trackID != trackID {
		t.Fatalf("PlaybackVariant() error = %v; authorizer = %+v", err, authorizer)
	}
}

func TestServiceFailsClosedWhenContentAccessIsUnavailable(t *testing.T) {
	store := &storeStub{asset: Asset{
		ID: uuid.New(), ContentTrackID: uuid.New(), Status: StatusReady,
		Variants: []AudioVariant{{ID: uuid.New(), BitrateKbps: 320}},
	}}
	_, _, err := NewService(store, &authorizerStub{err: errors.New("content unavailable")}).
		PlaybackVariant(context.Background(), store.asset.ID, AudioQualityHigh)
	if !errors.Is(err, ErrAccessUnavailable) {
		t.Fatalf("PlaybackVariant() error = %v, want %v", err, ErrAccessUnavailable)
	}
}

func TestServiceAllowsLegacyCatalogLinkedAsset(t *testing.T) {
	store := &storeStub{asset: Asset{
		ID: uuid.New(), TrackID: uuid.New(), Status: StatusReady,
		Variants: []AudioVariant{{ID: uuid.New(), BitrateKbps: 320}},
	}}
	_, _, err := NewService(store, nil).PlaybackVariant(
		context.Background(), store.asset.ID, AudioQualityHigh,
	)
	if err != nil {
		t.Fatalf("PlaybackVariant() error = %v", err)
	}
}
