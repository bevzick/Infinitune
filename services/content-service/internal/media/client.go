package media

import (
	"context"
	"errors"
	"fmt"
	"time"

	mediav1 "github.com/bevzick/Infinitune/services/content-service/api/media/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrNotFound    = errors.New("media asset not found")
	ErrUnavailable = errors.New("media service unavailable")
	ErrInvalid     = errors.New("invalid media response")
)

const requestTimeout = 3 * time.Second

type Status string

const (
	StatusUploaded   Status = "uploaded"
	StatusProcessing Status = "processing"
	StatusReady      Status = "ready"
	StatusFailed     Status = "failed"
)

type Asset struct {
	ID             uuid.UUID
	SourceUploadID uuid.UUID
	ContentTrackID uuid.UUID
	Status         Status
}

type Client struct {
	client mediav1.MediaServiceClient
}

func NewClient(client mediav1.MediaServiceClient) *Client {
	return &Client{client: client}
}

func (client *Client) AssetByUpload(
	ctx context.Context,
	uploadID uuid.UUID,
) (Asset, error) {
	if uploadID == uuid.Nil {
		return Asset{}, fmt.Errorf("upload_id must be a UUID: %w", ErrInvalid)
	}
	callCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	response, err := client.client.GetMediaAssetByUpload(
		callCtx, &mediav1.GetMediaAssetByUploadRequest{UploadId: uploadID.String()},
	)
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			return Asset{}, ErrNotFound
		case codes.Unavailable, codes.DeadlineExceeded, codes.Canceled:
			return Asset{}, fmt.Errorf("lookup media asset: %w", ErrUnavailable)
		default:
			return Asset{}, fmt.Errorf("lookup media asset: %w", err)
		}
	}
	asset, err := parseAsset(response)
	if err != nil {
		return Asset{}, err
	}
	if asset.SourceUploadID != uploadID {
		return Asset{}, fmt.Errorf("media upload mismatch: %w", ErrInvalid)
	}
	return asset, nil
}

func parseAsset(response *mediav1.MediaAsset) (Asset, error) {
	if response == nil {
		return Asset{}, fmt.Errorf("media response is empty: %w", ErrInvalid)
	}
	id, err := uuid.Parse(response.GetId())
	if err != nil || id == uuid.Nil {
		return Asset{}, fmt.Errorf("media asset id is invalid: %w", ErrInvalid)
	}
	uploadID, err := uuid.Parse(response.GetSourceUploadId())
	if err != nil || uploadID == uuid.Nil {
		return Asset{}, fmt.Errorf("media upload id is invalid: %w", ErrInvalid)
	}
	trackID, err := uuid.Parse(response.GetContentTrackId())
	if err != nil || trackID == uuid.Nil {
		return Asset{}, fmt.Errorf("media content track id is invalid: %w", ErrInvalid)
	}
	assetStatus, err := parseStatus(response.GetStatus())
	if err != nil {
		return Asset{}, err
	}
	return Asset{
		ID: id, SourceUploadID: uploadID, ContentTrackID: trackID, Status: assetStatus,
	}, nil
}

func parseStatus(value mediav1.MediaStatus) (Status, error) {
	switch value {
	case mediav1.MediaStatus_MEDIA_STATUS_UPLOADED:
		return StatusUploaded, nil
	case mediav1.MediaStatus_MEDIA_STATUS_PROCESSING:
		return StatusProcessing, nil
	case mediav1.MediaStatus_MEDIA_STATUS_READY:
		return StatusReady, nil
	case mediav1.MediaStatus_MEDIA_STATUS_FAILED:
		return StatusFailed, nil
	default:
		return "", fmt.Errorf("media status is invalid: %w", ErrInvalid)
	}
}
