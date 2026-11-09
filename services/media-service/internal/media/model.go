package media

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalid           = errors.New("invalid media request")
	ErrNotFound          = errors.New("media asset not found")
	ErrNotReady          = errors.New("media asset is not ready")
	ErrNotPlayable       = errors.New("media asset is not available for playback")
	ErrAccessUnavailable = errors.New("media playback access check is unavailable")
)

type Status string

const (
	StatusUploaded   Status = "uploaded"
	StatusProcessing Status = "processing"
	StatusReady      Status = "ready"
	StatusFailed     Status = "failed"
)

type Asset struct {
	ID                uuid.UUID
	TrackID           uuid.UUID
	ContentTrackID    uuid.UUID
	SourceUploadID    uuid.UUID
	OriginalObjectKey string
	OriginalFilename  string
	DurationMS        *int
	Status            Status
	Variants          []AudioVariant
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type AudioVariant struct {
	ID          uuid.UUID
	ObjectKey   string
	Codec       string
	Container   string
	BitrateKbps int
	SampleRate  int
	Channels    int
	SizeBytes   int64
	CreatedAt   time.Time
}

type Store interface {
	AssetByID(context.Context, uuid.UUID) (Asset, error)
	AssetBySourceUpload(context.Context, uuid.UUID) (Asset, error)
}

type PlaybackAuthorizer interface {
	CanStream(context.Context, uuid.UUID) (bool, error)
}

func (service *Service) AssetByUpload(
	ctx context.Context,
	uploadID uuid.UUID,
) (Asset, error) {
	if uploadID == uuid.Nil {
		return Asset{}, fmt.Errorf("upload_id must not be nil: %w", ErrInvalid)
	}
	return service.store.AssetBySourceUpload(ctx, uploadID)
}

type Service struct {
	store      Store
	authorizer PlaybackAuthorizer
}

func NewService(store Store, authorizer PlaybackAuthorizer) *Service {
	return &Service{store: store, authorizer: authorizer}
}

func (service *Service) Asset(ctx context.Context, id uuid.UUID) (Asset, error) {
	if id == uuid.Nil {
		return Asset{}, fmt.Errorf("media_asset_id must not be nil: %w", ErrInvalid)
	}
	return service.store.AssetByID(ctx, id)
}

func (service *Service) ReadyAsset(ctx context.Context, id uuid.UUID) (Asset, error) {
	asset, err := service.Asset(ctx, id)
	if err != nil {
		return Asset{}, err
	}
	if asset.Status != StatusReady {
		return Asset{}, fmt.Errorf("media asset status is %q: %w", asset.Status, ErrNotReady)
	}
	return asset, nil
}

func (service *Service) PlaybackVariant(
	ctx context.Context,
	id uuid.UUID,
) (Asset, AudioVariant, error) {
	asset, err := service.Asset(ctx, id)
	if err != nil {
		return Asset{}, AudioVariant{}, err
	}
	if asset.ContentTrackID != uuid.Nil {
		if service.authorizer == nil {
			return Asset{}, AudioVariant{}, ErrAccessUnavailable
		}
		allowed, accessErr := service.authorizer.CanStream(ctx, asset.ContentTrackID)
		if accessErr != nil {
			return Asset{}, AudioVariant{}, fmt.Errorf(
				"authorize content track playback: %w: %v", ErrAccessUnavailable, accessErr,
			)
		}
		if !allowed {
			return Asset{}, AudioVariant{}, ErrNotPlayable
		}
	} else if asset.TrackID == uuid.Nil {
		return Asset{}, AudioVariant{}, ErrNotPlayable
	}
	if asset.Status != StatusReady {
		return Asset{}, AudioVariant{}, fmt.Errorf("media asset status is %q: %w", asset.Status, ErrNotReady)
	}
	if len(asset.Variants) == 0 {
		return Asset{}, AudioVariant{}, fmt.Errorf("media asset has no audio variants: %w", ErrNotReady)
	}
	selected := asset.Variants[0]
	for _, variant := range asset.Variants[1:] {
		if variant.BitrateKbps > selected.BitrateKbps {
			selected = variant
		}
	}
	return asset, selected, nil
}
