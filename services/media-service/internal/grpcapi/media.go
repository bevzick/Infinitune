package grpcapi

import (
	"context"
	"errors"
	"time"

	mediav1 "github.com/bevzick/Infinitune/services/media-service/api/media/v1"
	"github.com/bevzick/Infinitune/services/media-service/internal/media"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const defaultRequestTimeout = 3 * time.Second

type Metadata interface {
	Asset(context.Context, uuid.UUID) (media.Asset, error)
	ReadyAsset(context.Context, uuid.UUID) (media.Asset, error)
}

type MediaServer struct {
	mediav1.UnimplementedMediaServiceServer
	metadata Metadata
	timeout  time.Duration
}

func NewMediaServer(metadata Metadata) *MediaServer {
	return &MediaServer{metadata: metadata, timeout: defaultRequestTimeout}
}

func (server *MediaServer) GetMediaAsset(
	ctx context.Context,
	request *mediav1.GetMediaAssetRequest,
) (*mediav1.MediaAsset, error) {
	id, err := parseAssetID(request.GetMediaAssetId())
	if err != nil {
		return nil, err
	}
	callCtx, cancel := context.WithTimeout(ctx, server.timeout)
	defer cancel()
	asset, err := server.metadata.Asset(callCtx, id)
	if err != nil {
		return nil, mapMediaError(err)
	}
	return assetResponse(asset), nil
}

func (server *MediaServer) GetReadyMediaAsset(
	ctx context.Context,
	request *mediav1.GetReadyMediaAssetRequest,
) (*mediav1.MediaAsset, error) {
	id, err := parseAssetID(request.GetMediaAssetId())
	if err != nil {
		return nil, err
	}
	callCtx, cancel := context.WithTimeout(ctx, server.timeout)
	defer cancel()
	asset, err := server.metadata.ReadyAsset(callCtx, id)
	if err != nil {
		return nil, mapMediaError(err)
	}
	return assetResponse(asset), nil
}

func parseAssetID(value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, status.Error(codes.InvalidArgument, "media_asset_id must be a UUID")
	}
	return id, nil
}

func mapMediaError(err error) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "media request timed out")
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "media request was canceled")
	case errors.Is(err, media.ErrInvalid):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, media.ErrNotFound):
		return status.Error(codes.NotFound, "media asset was not found")
	case errors.Is(err, media.ErrNotReady):
		return status.Error(codes.FailedPrecondition, "media asset is not ready")
	default:
		return status.Error(codes.Internal, "media metadata lookup failed")
	}
}

func assetResponse(asset media.Asset) *mediav1.MediaAsset {
	variants := make([]*mediav1.AudioVariant, 0, len(asset.Variants))
	for _, variant := range asset.Variants {
		variants = append(variants, &mediav1.AudioVariant{
			Id: variant.ID.String(), ObjectKey: variant.ObjectKey,
			Codec: variant.Codec, Container: variant.Container,
			BitrateKbps: int32(variant.BitrateKbps), SampleRate: int32(variant.SampleRate),
			Channels: int32(variant.Channels), SizeBytes: variant.SizeBytes,
			CreatedAt: timestamppb.New(variant.CreatedAt),
		})
	}
	response := &mediav1.MediaAsset{
		Id: asset.ID.String(), TrackId: asset.TrackID.String(),
		SourceUploadId:    asset.SourceUploadID.String(),
		OriginalObjectKey: asset.OriginalObjectKey,
		OriginalFilename:  asset.OriginalFilename,
		Status:            assetStatus(asset.Status), Variants: variants,
		CreatedAt: timestamppb.New(asset.CreatedAt), UpdatedAt: timestamppb.New(asset.UpdatedAt),
	}
	if asset.DurationMS != nil {
		value := int32(*asset.DurationMS)
		response.DurationMs = &value
	}
	return response
}

func assetStatus(value media.Status) mediav1.MediaStatus {
	switch value {
	case media.StatusUploaded:
		return mediav1.MediaStatus_MEDIA_STATUS_UPLOADED
	case media.StatusProcessing:
		return mediav1.MediaStatus_MEDIA_STATUS_PROCESSING
	case media.StatusReady:
		return mediav1.MediaStatus_MEDIA_STATUS_READY
	case media.StatusFailed:
		return mediav1.MediaStatus_MEDIA_STATUS_FAILED
	default:
		return mediav1.MediaStatus_MEDIA_STATUS_UNSPECIFIED
	}
}
