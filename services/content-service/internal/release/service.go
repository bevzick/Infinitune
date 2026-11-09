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
	Update(context.Context, Draft) (Draft, error)
	Withdraw(context.Context, uuid.UUID) (Release, error)
	Archive(context.Context, uuid.UUID) (Release, error)
}

type Authorizer interface {
	Authorize(context.Context, uuid.UUID, uuid.UUID, creator.Permission) error
}

type CatalogArchiver interface {
	Archive(context.Context, uuid.UUID) error
}

type Service struct {
	store      Store
	authorizer Authorizer
	catalog    CatalogArchiver
}

func NewService(store Store, authorizer Authorizer, catalog ...CatalogArchiver) *Service {
	service := &Service{store: store, authorizer: authorizer}
	if len(catalog) > 0 {
		service.catalog = catalog[0]
	}
	return service
}

func (service *Service) Update(
	ctx context.Context,
	actorID uuid.UUID,
	releaseID uuid.UUID,
	input UpdateInput,
) (Draft, error) {
	if releaseID == uuid.Nil {
		return Draft{}, validationError("release_id", "must be a UUID")
	}
	current, err := service.store.ByID(ctx, releaseID)
	if err != nil {
		return Draft{}, fmt.Errorf("read updated release: %w", err)
	}
	if err = service.authorizer.Authorize(
		ctx, actorID, current.Release.CreatorID, creator.PermissionContentEdit,
	); err != nil {
		return Draft{}, fmt.Errorf("authorize release update: %w", err)
	}
	if !editableStatus(current.Release.Status) {
		return Draft{}, ErrConflict
	}
	updated, err := updatedDraft(current, input)
	if err != nil {
		return Draft{}, err
	}
	result, err := service.store.Update(ctx, updated)
	if err != nil {
		return Draft{}, fmt.Errorf("update release: %w", err)
	}
	return result, nil
}

func (service *Service) Withdraw(
	ctx context.Context,
	actorID uuid.UUID,
	releaseID uuid.UUID,
) (Release, error) {
	current, err := service.authorizedRelease(
		ctx, actorID, releaseID, creator.PermissionContentPublish,
	)
	if err != nil {
		return Release{}, err
	}
	if current.Release.Status == StatusDraft {
		return current.Release, nil
	}
	if current.Release.Status != StatusPendingReview && current.Release.Status != StatusApproved {
		return Release{}, ErrConflict
	}
	result, err := service.store.Withdraw(ctx, releaseID)
	if err != nil {
		return Release{}, fmt.Errorf("withdraw release: %w", err)
	}
	return result, nil
}

func (service *Service) Archive(
	ctx context.Context,
	actorID uuid.UUID,
	releaseID uuid.UUID,
) (Release, error) {
	current, err := service.authorizedRelease(
		ctx, actorID, releaseID, creator.PermissionContentPublish,
	)
	if err != nil {
		return Release{}, err
	}
	if current.Release.Status == StatusArchived {
		return current.Release, nil
	}
	if current.Release.Status == StatusPublished {
		if service.catalog == nil {
			return Release{}, ErrCatalogUnavailable
		}
		if err = service.catalog.Archive(ctx, releaseID); err != nil {
			return Release{}, fmt.Errorf("archive catalog release: %w: %v", ErrCatalogUnavailable, err)
		}
	}
	result, err := service.store.Archive(ctx, releaseID)
	if err != nil {
		return Release{}, fmt.Errorf("archive release: %w", err)
	}
	return result, nil
}

func (service *Service) authorizedRelease(
	ctx context.Context,
	actorID uuid.UUID,
	releaseID uuid.UUID,
	permission creator.Permission,
) (Draft, error) {
	if releaseID == uuid.Nil {
		return Draft{}, validationError("release_id", "must be a UUID")
	}
	current, err := service.store.ByID(ctx, releaseID)
	if err != nil {
		return Draft{}, fmt.Errorf("read release lifecycle state: %w", err)
	}
	if err = service.authorizer.Authorize(
		ctx, actorID, current.Release.CreatorID, permission,
	); err != nil {
		return Draft{}, fmt.Errorf("authorize release lifecycle operation: %w", err)
	}
	return current, nil
}

func editableStatus(status Status) bool {
	switch status {
	case StatusDraft, StatusRejected, StatusChangesRequested:
		return true
	default:
		return false
	}
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
