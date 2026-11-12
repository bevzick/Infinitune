package media

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

type AssetStore interface {
	AssetByUpload(context.Context, uuid.UUID) (Asset, error)
}

type ProjectedStore struct {
	projection AssetStore
	legacy     AssetStore
}

func NewProjectedStore(projection, legacy AssetStore) *ProjectedStore {
	return &ProjectedStore{projection: projection, legacy: legacy}
}

func (store *ProjectedStore) AssetByUpload(ctx context.Context, uploadID uuid.UUID) (Asset, error) {
	asset, err := store.projection.AssetByUpload(ctx, uploadID)
	if !errors.Is(err, ErrNotFound) {
		return asset, err
	}
	return store.legacy.AssetByUpload(ctx, uploadID)
}
