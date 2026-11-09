package media

import (
	"context"
	"errors"
	"testing"

	mediav1 "github.com/bevzick/Infinitune/services/content-service/api/media/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestClientLoadsAssetByUpload(t *testing.T) {
	uploadID, trackID, assetID := uuid.New(), uuid.New(), uuid.New()
	client := NewClient(mediaClientStub{response: &mediav1.MediaAsset{
		Id: assetID.String(), SourceUploadId: uploadID.String(),
		ContentTrackId: trackID.String(),
		Status:         mediav1.MediaStatus_MEDIA_STATUS_READY,
	}})
	asset, err := client.AssetByUpload(context.Background(), uploadID)
	if err != nil || asset.ID != assetID || asset.SourceUploadID != uploadID ||
		asset.ContentTrackID != trackID || asset.Status != StatusReady {
		t.Fatalf("AssetByUpload() = %+v, %v", asset, err)
	}
}

func TestClientMapsMediaErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{
		{name: "missing", err: status.Error(codes.NotFound, "missing"), want: ErrNotFound},
		{name: "unavailable", err: status.Error(codes.Unavailable, "down"), want: ErrUnavailable},
		{name: "deadline", err: status.Error(codes.DeadlineExceeded, "slow"), want: ErrUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewClient(mediaClientStub{err: test.err}).AssetByUpload(
				context.Background(), uuid.New(),
			)
			if !errors.Is(err, test.want) {
				t.Fatalf("AssetByUpload() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestClientRejectsMalformedOrMismatchedResponse(t *testing.T) {
	uploadID := uuid.New()
	tests := []struct {
		name     string
		response *mediav1.MediaAsset
	}{
		{name: "nil response"},
		{name: "invalid asset id", response: &mediav1.MediaAsset{
			Id: "invalid", SourceUploadId: uploadID.String(),
			ContentTrackId: uuid.NewString(), Status: mediav1.MediaStatus_MEDIA_STATUS_READY,
		}},
		{name: "mismatched upload", response: &mediav1.MediaAsset{
			Id: uuid.NewString(), SourceUploadId: uuid.NewString(),
			ContentTrackId: uuid.NewString(), Status: mediav1.MediaStatus_MEDIA_STATUS_READY,
		}},
		{name: "unspecified status", response: &mediav1.MediaAsset{
			Id: uuid.NewString(), SourceUploadId: uploadID.String(),
			ContentTrackId: uuid.NewString(),
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewClient(mediaClientStub{response: test.response}).AssetByUpload(
				context.Background(), uploadID,
			)
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("AssetByUpload() error = %v", err)
			}
		})
	}
}

type mediaClientStub struct {
	response *mediav1.MediaAsset
	err      error
}

func (stub mediaClientStub) GetMediaAssetByUpload(
	context.Context,
	*mediav1.GetMediaAssetByUploadRequest,
	...grpc.CallOption,
) (*mediav1.MediaAsset, error) {
	return stub.response, stub.err
}

var _ mediav1.MediaServiceClient = mediaClientStub{}
