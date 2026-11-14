package grpcapi

import (
	"context"
	"errors"
	"testing"
	"time"

	mediav1 "github.com/bevzick/Infinitune/services/media-service/api/media/v1"
	"github.com/bevzick/Infinitune/services/media-service/internal/media"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type metadataStub struct {
	asset media.Asset
	err   error
}

func (stub metadataStub) Asset(context.Context, uuid.UUID) (media.Asset, error) {
	return stub.asset, stub.err
}

func (stub metadataStub) ReadyAsset(context.Context, uuid.UUID) (media.Asset, error) {
	return stub.asset, stub.err
}

func (stub metadataStub) AssetByUpload(context.Context, uuid.UUID) (media.Asset, error) {
	return stub.asset, stub.err
}

func TestGetMediaAssetMapsMetadata(t *testing.T) {
	id := uuid.New()
	trackID := uuid.New()
	duration := 123000
	now := time.Now().UTC().Truncate(time.Second)
	variantID := uuid.New()
	server := NewMediaServer(metadataStub{asset: media.Asset{
		ID: id, TrackID: trackID, SourceUploadID: uuid.New(),
		OriginalObjectKey: "original/file.wav", OriginalFilename: "file.wav",
		DurationMS: &duration, Status: media.StatusReady, CreatedAt: now, UpdatedAt: now,
		Variants: []media.AudioVariant{{
			ID: variantID, ObjectKey: "variants/file.m4a", Codec: "aac", Container: "m4a",
			BitrateKbps: 256, SampleRate: 48000, Channels: 2, SizeBytes: 4096, CreatedAt: now,
		}},
	}})
	response, err := server.GetMediaAsset(context.Background(), &mediav1.GetMediaAssetRequest{
		MediaAssetId: id.String(),
	})
	if err != nil {
		t.Fatalf("GetMediaAsset() error = %v", err)
	}
	if response.GetId() != id.String() || response.GetTrackId() != trackID.String() ||
		response.GetDurationMs() != 123000 || response.DurationMs == nil ||
		response.GetStatus() != mediav1.MediaStatus_MEDIA_STATUS_READY ||
		len(response.GetVariants()) != 1 || response.GetVariants()[0].GetId() != variantID.String() {
		t.Fatalf("GetMediaAsset() = %+v", response)
	}
}

func TestGetMediaAssetOmitsUploadIDForExternalImport(t *testing.T) {
	response, err := NewMediaServer(metadataStub{asset: media.Asset{
		ID: uuid.New(), ContentTrackID: uuid.New(), Status: media.StatusUploaded,
	}}).GetMediaAsset(context.Background(), &mediav1.GetMediaAssetRequest{
		MediaAssetId: uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("GetMediaAsset() error = %v", err)
	}
	if response.GetSourceUploadId() != "" {
		t.Fatalf("SourceUploadId = %q, want empty", response.GetSourceUploadId())
	}
}

func TestMediaServerReturnsControlledErrors(t *testing.T) {
	tests := []struct {
		name string
		id   string
		err  error
		want codes.Code
	}{
		{name: "invalid id", id: "invalid", want: codes.InvalidArgument},
		{name: "missing", id: uuid.NewString(), err: media.ErrNotFound, want: codes.NotFound},
		{name: "not ready", id: uuid.NewString(), err: media.ErrNotReady, want: codes.FailedPrecondition},
		{name: "internal", id: uuid.NewString(), err: errors.New("database unavailable"), want: codes.Internal},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := NewMediaServer(metadataStub{err: test.err})
			_, err := server.GetReadyMediaAsset(context.Background(),
				&mediav1.GetReadyMediaAssetRequest{MediaAssetId: test.id})
			if status.Code(err) != test.want {
				t.Fatalf("GetReadyMediaAsset() code = %v, want %v", status.Code(err), test.want)
			}
		})
	}
}

func TestGetMediaAssetByUploadReturnsMatchingAsset(t *testing.T) {
	uploadID, trackID := uuid.New(), uuid.New()
	response, err := NewMediaServer(metadataStub{asset: media.Asset{
		ID: uuid.New(), SourceUploadID: uploadID, ContentTrackID: trackID,
		Status: media.StatusReady,
	}}).GetMediaAssetByUpload(context.Background(), &mediav1.GetMediaAssetByUploadRequest{
		UploadId: uploadID.String(),
	})
	if err != nil || response.GetSourceUploadId() != uploadID.String() ||
		response.GetContentTrackId() != trackID.String() ||
		response.GetStatus() != mediav1.MediaStatus_MEDIA_STATUS_READY {
		t.Fatalf("GetMediaAssetByUpload() = %+v, %v", response, err)
	}
	_, err = NewMediaServer(metadataStub{}).GetMediaAssetByUpload(
		context.Background(), &mediav1.GetMediaAssetByUploadRequest{UploadId: "invalid"},
	)
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("GetMediaAssetByUpload(invalid) code = %v", status.Code(err))
	}
}
