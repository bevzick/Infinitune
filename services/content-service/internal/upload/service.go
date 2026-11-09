package upload

import (
	"context"
	"errors"
	"fmt"

	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/google/uuid"
)

type Store interface {
	TrackTarget(context.Context, uuid.UUID) (TrackTarget, error)
	ByRequest(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (Session, error)
	Create(context.Context, Session) (Session, bool, error)
	ByID(context.Context, uuid.UUID) (Session, error)
	Transition(context.Context, uuid.UUID, Status, Status) (Session, error)
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

func (service *Service) CreateSession(
	ctx context.Context,
	actorID uuid.UUID,
	input CreateInput,
) (Session, error) {
	validated, err := validateCreateInput(input)
	if err != nil {
		return Session{}, err
	}
	target, err := service.store.TrackTarget(ctx, validated.ReleaseTrackID)
	if err != nil {
		return Session{}, fmt.Errorf("read upload track target: %w", err)
	}
	if err = service.authorizer.Authorize(
		ctx, actorID, target.CreatorID, creator.PermissionContentUpload,
	); err != nil {
		return Session{}, fmt.Errorf("authorize track upload: %w", err)
	}
	existing, err := service.store.ByRequest(
		ctx, target.TrackID, actorID, validated.IdempotencyKey,
	)
	if err == nil {
		requested := Session{
			ReleaseTrackID: target.TrackID, RequestedByUserID: actorID,
			IdempotencyKey: validated.IdempotencyKey,
			OriginalName:   validated.OriginalName, MIMEType: validated.MIMEType,
			SizeBytes: validated.SizeBytes,
		}
		if !sameRequest(existing, requested) {
			return Session{}, ErrIdempotencyConflict
		}
		return existing, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return Session{}, fmt.Errorf("find idempotent upload session: %w", err)
	}
	if target.ReleaseStatus != release.StatusDraft &&
		target.ReleaseStatus != release.StatusRejected &&
		target.ReleaseStatus != release.StatusChangesRequested {
		return Session{}, ErrReleaseLocked
	}
	uploadID := uuid.New()
	session := Session{
		ID: uploadID, ReleaseTrackID: target.TrackID, RequestedByUserID: actorID,
		IdempotencyKey: validated.IdempotencyKey,
		ObjectKey: fmt.Sprintf(
			"creator-content/%s/%s/%s/%s/source%s",
			target.CreatorID, target.ReleaseID, target.TrackID, uploadID,
			validated.Extension,
		),
		OriginalName: validated.OriginalName, MIMEType: validated.MIMEType,
		SizeBytes: validated.SizeBytes, Status: StatusPending,
	}
	stored, created, err := service.store.Create(ctx, session)
	if err != nil {
		return Session{}, fmt.Errorf("create upload session: %w", err)
	}
	if !created && !sameRequest(stored, session) {
		return Session{}, ErrIdempotencyConflict
	}
	return stored, nil
}

func (service *Service) SessionForUser(
	ctx context.Context,
	actorID uuid.UUID,
	uploadID uuid.UUID,
) (Session, error) {
	if actorID == uuid.Nil {
		return Session{}, creator.ErrForbidden
	}
	if uploadID == uuid.Nil {
		return Session{}, validationError("upload_id", "must be a UUID")
	}
	session, err := service.store.ByID(ctx, uploadID)
	if err != nil {
		return Session{}, fmt.Errorf("read upload session: %w", err)
	}
	if session.RequestedByUserID != actorID {
		return Session{}, creator.ErrForbidden
	}
	return session, nil
}

func (service *Service) Transition(
	ctx context.Context,
	uploadID uuid.UUID,
	target Status,
) (Session, error) {
	if uploadID == uuid.Nil {
		return Session{}, validationError("upload_id", "must be a UUID")
	}
	current, err := service.store.ByID(ctx, uploadID)
	if err != nil {
		return Session{}, fmt.Errorf("read upload session: %w", err)
	}
	if current.Status == target {
		return current, nil
	}
	if !canTransition(current.Status, target) {
		return Session{}, ErrInvalidTransition
	}
	updated, err := service.store.Transition(ctx, uploadID, current.Status, target)
	if err != nil {
		return Session{}, fmt.Errorf("transition upload session: %w", err)
	}
	return updated, nil
}

func sameRequest(left, right Session) bool {
	return left.ReleaseTrackID == right.ReleaseTrackID &&
		left.RequestedByUserID == right.RequestedByUserID &&
		left.IdempotencyKey == right.IdempotencyKey &&
		left.OriginalName == right.OriginalName && left.MIMEType == right.MIMEType &&
		left.SizeBytes == right.SizeBytes
}

func canTransition(current, target Status) bool {
	switch current {
	case StatusPending:
		return target == StatusUploading || target == StatusFailed
	case StatusUploading:
		return target == StatusCompleted || target == StatusFailed
	case StatusFailed:
		return target == StatusPending
	case StatusCompleted:
		return false
	default:
		return false
	}
}
