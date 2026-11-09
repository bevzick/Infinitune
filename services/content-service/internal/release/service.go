package release

import (
	"context"
	"fmt"

	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/google/uuid"
)

type Store interface {
	Create(context.Context, Draft) (Draft, error)
	ByID(context.Context, uuid.UUID) (Draft, error)
}

type Authorizer interface {
	Authorize(context.Context, uuid.UUID, uuid.UUID, creator.Permission) error
}

type Service struct {
	store      Store
	authorizer Authorizer
}

func NewService(store Store, authorizer Authorizer) *Service {
	return &Service{store: store, authorizer: authorizer}
}

func (service *Service) CreateDraft(
	ctx context.Context,
	actorID uuid.UUID,
	input CreateDraftInput,
) (Draft, error) {
	draft, err := newDraft(input)
	if err != nil {
		return Draft{}, err
	}
	if err = service.authorizer.Authorize(
		ctx, actorID, draft.Release.CreatorID, creator.PermissionContentEdit,
	); err != nil {
		return Draft{}, fmt.Errorf("authorize draft release creation: %w", err)
	}
	created, err := service.store.Create(ctx, draft)
	if err != nil {
		return Draft{}, fmt.Errorf("create draft release: %w", err)
	}
	return created, nil
}

func (service *Service) ByID(
	ctx context.Context,
	actorID uuid.UUID,
	releaseID uuid.UUID,
) (Draft, error) {
	if releaseID == uuid.Nil {
		return Draft{}, validationError("release_id", "must be a UUID")
	}
	draft, err := service.store.ByID(ctx, releaseID)
	if err != nil {
		return Draft{}, fmt.Errorf("read draft release: %w", err)
	}
	if err = service.authorizer.Authorize(
		ctx, actorID, draft.Release.CreatorID, creator.PermissionAccountRead,
	); err != nil {
		return Draft{}, fmt.Errorf("authorize draft release read: %w", err)
	}
	return draft, nil
}
