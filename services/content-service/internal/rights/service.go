package rights

import (
	"context"
	"fmt"

	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/google/uuid"
)

type Store interface {
	TrackTarget(context.Context, uuid.UUID) (TrackTarget, error)
	RightTarget(context.Context, uuid.UUID) (RightTarget, error)
	UpsertRight(context.Context, Right) (Right, error)
	ListRights(context.Context, uuid.UUID) ([]Right, error)
	DeleteRight(context.Context, uuid.UUID) error
}

type AccessChecker interface {
	Check(
		context.Context, uuid.UUID, uuid.UUID, creator.Permission,
	) (creator.AccessDecision, error)
}

type Service struct {
	store  Store
	access AccessChecker
}

func NewService(store Store, access AccessChecker) *Service {
	return &Service{store: store, access: access}
}

func (service *Service) Set(
	ctx context.Context,
	actorID uuid.UUID,
	input SetInput,
) (Right, error) {
	validated, err := validateSetInput(input)
	if err != nil {
		return Right{}, err
	}
	target, err := service.store.TrackTarget(ctx, validated.ReleaseTrackID)
	if err != nil {
		return Right{}, fmt.Errorf("read rights track target: %w", err)
	}
	if err = service.authorize(
		ctx, actorID, target.CreatorID, creator.PermissionContentPublish,
	); err != nil {
		return Right{}, fmt.Errorf("authorize track rights update: %w", err)
	}
	if !rightsMutable(target.ReleaseStatus) {
		return Right{}, ErrImmutable
	}
	right := Right{
		ID: uuid.New(), ReleaseTrackID: target.TrackID,
		TerritoryCode: validated.TerritoryCode, Type: validated.Type,
		ValidFrom: validated.ValidFrom, ValidUntil: validated.ValidUntil,
		GrantedByUserID: actorID,
	}
	stored, err := service.store.UpsertRight(ctx, right)
	if err != nil {
		return Right{}, fmt.Errorf("store track rights: %w", err)
	}
	return stored, nil
}

func (service *Service) List(
	ctx context.Context,
	actorID uuid.UUID,
	trackID uuid.UUID,
) ([]Right, error) {
	if trackID == uuid.Nil {
		return nil, validationError("release_track_id", "must be a UUID")
	}
	target, err := service.store.TrackTarget(ctx, trackID)
	if err != nil {
		return nil, fmt.Errorf("read rights track target: %w", err)
	}
	if err = service.authorize(
		ctx, actorID, target.CreatorID, creator.PermissionAccountRead,
	); err != nil {
		return nil, fmt.Errorf("authorize track rights read: %w", err)
	}
	result, err := service.store.ListRights(ctx, trackID)
	if err != nil {
		return nil, fmt.Errorf("list track rights: %w", err)
	}
	return result, nil
}

func (service *Service) Delete(
	ctx context.Context,
	actorID uuid.UUID,
	rightID uuid.UUID,
) error {
	if rightID == uuid.Nil {
		return validationError("right_id", "must be a UUID")
	}
	target, err := service.store.RightTarget(ctx, rightID)
	if err != nil {
		return fmt.Errorf("read deleted track rights: %w", err)
	}
	if err = service.authorize(
		ctx, actorID, target.Track.CreatorID, creator.PermissionContentPublish,
	); err != nil {
		return fmt.Errorf("authorize track rights deletion: %w", err)
	}
	if !rightsMutable(target.Track.ReleaseStatus) {
		return ErrImmutable
	}
	if err = service.store.DeleteRight(ctx, rightID); err != nil {
		return fmt.Errorf("delete track rights: %w", err)
	}
	return nil
}

func (service *Service) authorize(
	ctx context.Context,
	actorID uuid.UUID,
	creatorID uuid.UUID,
	permission creator.Permission,
) error {
	if actorID == uuid.Nil {
		return creator.ErrForbidden
	}
	decision, err := service.access.Check(ctx, actorID, creatorID, permission)
	if err != nil {
		return err
	}
	if decision.Allowed {
		return nil
	}
	if permission == creator.PermissionContentPublish && decision.Role != "" &&
		decision.VerificationStatus != creator.VerificationVerified {
		return creator.ErrVerificationRequired
	}
	return creator.ErrForbidden
}

func rightsMutable(status release.Status) bool {
	switch status {
	case release.StatusDraft, release.StatusUploading,
		release.StatusProcessing, release.StatusRejected, release.StatusChangesRequested:
		return true
	default:
		return false
	}
}
