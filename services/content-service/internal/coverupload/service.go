package coverupload

import (
	"context"
	"errors"
	"fmt"

	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/google/uuid"
)

type Store interface {
	ReleaseTarget(context.Context, uuid.UUID) (ReleaseTarget, error)
	ByRequest(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (Session, error)
	Create(context.Context, Session) (Session, bool, error)
	ByID(context.Context, uuid.UUID) (Session, error)
	Transition(context.Context, uuid.UUID, Status, Status) (Session, error)
	Complete(context.Context, uuid.UUID) (Session, error)
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

func (service *Service) CreateSession(ctx context.Context, actorID uuid.UUID, input CreateInput) (Session, error) {
	if actorID == uuid.Nil {
		return Session{}, creator.ErrForbidden
	}
	validated, err := validateCreateInput(input)
	if err != nil {
		return Session{}, err
	}
	target, err := service.store.ReleaseTarget(ctx, validated.ReleaseID)
	if err != nil {
		return Session{}, fmt.Errorf("read cover release target: %w", err)
	}
	if err = service.authorizer.Authorize(ctx, actorID, target.CreatorID, creator.PermissionContentUpload); err != nil {
		return Session{}, fmt.Errorf("authorize cover upload: %w", err)
	}
	if !editableStatus(target.Status) {
		return Session{}, ErrReleaseLocked
	}
	existing, err := service.store.ByRequest(ctx, target.ReleaseID, actorID, validated.IdempotencyKey)
	if err == nil {
		if !sameRequest(existing, validated, actorID) {
			return Session{}, ErrIdempotencyConflict
		}
		return existing, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return Session{}, fmt.Errorf("find cover upload request: %w", err)
	}
	uploadID := uuid.New()
	session := Session{
		ID: uploadID, ReleaseID: target.ReleaseID, RequestedByUserID: actorID,
		IdempotencyKey: validated.IdempotencyKey,
		ObjectKey: fmt.Sprintf("creator-content/%s/%s/covers/%s/cover%s",
			target.CreatorID, target.ReleaseID, uploadID, validated.Extension),
		OriginalName: validated.OriginalName, MIMEType: validated.MIMEType,
		SizeBytes: validated.SizeBytes, Status: StatusPending,
	}
	stored, created, err := service.store.Create(ctx, session)
	if err != nil {
		return Session{}, fmt.Errorf("create cover upload session: %w", err)
	}
	if !created && !sameRequest(stored, validated, actorID) {
		return Session{}, ErrIdempotencyConflict
	}
	return stored, nil
}

func (service *Service) SessionForUser(ctx context.Context, actorID, uploadID uuid.UUID) (Session, error) {
	if actorID == uuid.Nil {
		return Session{}, creator.ErrForbidden
	}
	if uploadID == uuid.Nil {
		return Session{}, invalidField("upload_id", "must be a UUID")
	}
	session, err := service.store.ByID(ctx, uploadID)
	if err != nil {
		return Session{}, fmt.Errorf("read cover upload session: %w", err)
	}
	if session.RequestedByUserID != actorID {
		return Session{}, creator.ErrForbidden
	}
	target, err := service.store.ReleaseTarget(ctx, session.ReleaseID)
	if err != nil {
		return Session{}, fmt.Errorf("read cover release target: %w", err)
	}
	if err = service.authorizer.Authorize(ctx, actorID, target.CreatorID, creator.PermissionContentUpload); err != nil {
		return Session{}, fmt.Errorf("authorize cover session: %w", err)
	}
	if !editableStatus(target.Status) {
		return Session{}, ErrReleaseLocked
	}
	return session, nil
}

func (service *Service) Transition(ctx context.Context, uploadID uuid.UUID, status Status) (Session, error) {
	if uploadID == uuid.Nil {
		return Session{}, invalidField("upload_id", "must be a UUID")
	}
	current, err := service.store.ByID(ctx, uploadID)
	if err != nil {
		return Session{}, fmt.Errorf("read cover upload session: %w", err)
	}
	if current.Status == status {
		return current, nil
	}
	// Completion must atomically attach the cover to its release.
	if status == StatusCompleted {
		return Session{}, ErrInvalidTransition
	}
	if status == StatusUploading {
		target, err := service.store.ReleaseTarget(ctx, current.ReleaseID)
		if err != nil {
			return Session{}, fmt.Errorf("read cover release target: %w", err)
		}
		if !editableStatus(target.Status) {
			return Session{}, ErrReleaseLocked
		}
	}
	if !canTransition(current.Status, status) {
		return Session{}, ErrInvalidTransition
	}
	updated, err := service.store.Transition(ctx, uploadID, current.Status, status)
	if err != nil {
		return Session{}, fmt.Errorf("transition cover upload session: %w", err)
	}
	return updated, nil
}

// Complete marks the upload completed and assigns its object key to the
// release in one Content-owned database transaction. Only Media calls this.
func (service *Service) Complete(ctx context.Context, uploadID uuid.UUID) (Session, error) {
	if uploadID == uuid.Nil {
		return Session{}, invalidField("upload_id", "must be a UUID")
	}
	session, err := service.store.Complete(ctx, uploadID)
	if err != nil {
		return Session{}, fmt.Errorf("complete cover session: %w", err)
	}
	return session, nil
}

func editableStatus(status release.Status) bool {
	return status == release.StatusDraft || status == release.StatusRejected || status == release.StatusChangesRequested
}

func sameRequest(session Session, requested validatedInput, actorID uuid.UUID) bool {
	return session.ReleaseID == requested.ReleaseID && session.RequestedByUserID == actorID &&
		session.IdempotencyKey == requested.IdempotencyKey && session.OriginalName == requested.OriginalName &&
		session.MIMEType == requested.MIMEType && session.SizeBytes == requested.SizeBytes
}

func canTransition(current, next Status) bool {
	switch current {
	case StatusPending:
		return next == StatusUploading || next == StatusFailed
	case StatusUploading:
		return next == StatusFailed
	case StatusFailed:
		return next == StatusPending
	default:
		return false
	}
}
