package media

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalid  = errors.New("invalid media request")
	ErrNotFound = errors.New("media asset not found")
	ErrNotReady = errors.New("media asset is not ready")
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
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
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
	asset, err := service.ReadyAsset(ctx, id)
	if err != nil {
		return Asset{}, AudioVariant{}, err
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
