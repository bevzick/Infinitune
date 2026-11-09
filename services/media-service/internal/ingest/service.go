package ingest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"strings"
	"time"

	"github.com/bevzick/Infinitune/services/media-service/internal/content"
	"github.com/bevzick/Infinitune/services/media-service/internal/media"
	"github.com/google/uuid"
)

var (
	ErrInvalid      = errors.New("invalid media ingest request")
	ErrInProgress   = errors.New("media ingest is already in progress")
	ErrInconsistent = errors.New("media ingest state is inconsistent")
	ErrStorage      = errors.New("media object storage failed")
)

const compensationTimeout = 5 * time.Second

type ContentUploads interface {
	Session(context.Context, uuid.UUID, uuid.UUID) (content.Session, error)
	Transition(context.Context, uuid.UUID, content.Status) (content.Session, error)
}

type AssetStore interface {
	AssetBySourceUpload(context.Context, uuid.UUID) (media.Asset, error)
	CreateFromUpload(context.Context, media.Asset) (media.Asset, bool, error)
	EnqueueProcessing(context.Context, uuid.UUID) error
}

type ObjectStore interface {
	Put(context.Context, string, io.Reader, int64, string) error
	Delete(context.Context, string) error
}

type Service struct {
	content ContentUploads
	assets  AssetStore
	objects ObjectStore
}

func NewService(content ContentUploads, assets AssetStore, objects ObjectStore) *Service {
	return &Service{content: content, assets: assets, objects: objects}
}

func (service *Service) Ingest(
	ctx context.Context,
	actorID uuid.UUID,
	uploadID uuid.UUID,
	contentType string,
	sizeBytes int64,
	body io.Reader,
) (media.Asset, bool, error) {
	if actorID == uuid.Nil {
		return media.Asset{}, false, content.ErrForbidden
	}
	if uploadID == uuid.Nil {
		return media.Asset{}, false, fmt.Errorf("upload_id must be a UUID: %w", ErrInvalid)
	}
	if body == nil {
		return media.Asset{}, false, fmt.Errorf("audio body is required: %w", ErrInvalid)
	}
	session, err := service.content.Session(ctx, actorID, uploadID)
	if err != nil {
		return media.Asset{}, false, fmt.Errorf("read content upload session: %w", err)
	}
	mediaType, parameters, parseErr := mime.ParseMediaType(strings.TrimSpace(contentType))
	if parseErr != nil || len(parameters) != 0 ||
		!strings.EqualFold(mediaType, session.MIMEType) {
		return media.Asset{}, false, fmt.Errorf("content type does not match upload session: %w", ErrInvalid)
	}
	if sizeBytes != session.SizeBytes {
		return media.Asset{}, false, fmt.Errorf("content length does not match upload session: %w", ErrInvalid)
	}

	existing, findErr := service.assets.AssetBySourceUpload(ctx, uploadID)
	if findErr == nil {
		if !matchesSession(existing, session) {
			return media.Asset{}, false, ErrInconsistent
		}
		if err = service.completeExisting(ctx, session); err != nil {
			return media.Asset{}, false, err
		}
		if err = service.enqueueProcessing(ctx, existing.ID); err != nil {
			return media.Asset{}, false, fmt.Errorf("enqueue media processing: %w", err)
		}
		return existing, false, nil
	}
	if !errors.Is(findErr, media.ErrNotFound) {
		return media.Asset{}, false, fmt.Errorf("find media asset by upload: %w", findErr)
	}

	session, err = service.start(ctx, session)
	if err != nil {
		return media.Asset{}, false, err
	}
	if err = service.objects.Put(
		ctx, session.ObjectKey, body, session.SizeBytes, session.MIMEType,
	); err != nil {
		compensationCtx, cancel := compensationContext(ctx)
		defer cancel()
		transitionErr := service.fail(compensationCtx, session.ID)
		return media.Asset{}, false, errors.Join(
			fmt.Errorf("store original audio: %w: %v", ErrStorage, err), transitionErr,
		)
	}
	asset := media.Asset{
		ID: uuid.New(), ContentTrackID: session.ReleaseTrackID,
		SourceUploadID: session.ID, OriginalObjectKey: session.ObjectKey,
		OriginalFilename: session.OriginalName, Status: media.StatusUploaded,
	}
	stored, created, err := service.assets.CreateFromUpload(ctx, asset)
	if err != nil {
		compensationCtx, cancel := compensationContext(ctx)
		defer cancel()
		deleteErr := service.objects.Delete(compensationCtx, session.ObjectKey)
		transitionErr := service.fail(compensationCtx, session.ID)
		return media.Asset{}, false, errors.Join(
			fmt.Errorf("create media asset: %w", err), deleteErr, transitionErr,
		)
	}
	if !matchesSession(stored, session) {
		return media.Asset{}, false, ErrInconsistent
	}
	completionCtx, cancel := compensationContext(ctx)
	defer cancel()
	if _, err = service.content.Transition(
		completionCtx, session.ID, content.StatusCompleted,
	); err != nil {
		return media.Asset{}, false, fmt.Errorf("complete content upload session: %w", err)
	}
	if err = service.enqueueProcessing(ctx, stored.ID); err != nil {
		return media.Asset{}, false, fmt.Errorf("enqueue media processing: %w", err)
	}
	return stored, created, nil
}

func (service *Service) enqueueProcessing(ctx context.Context, assetID uuid.UUID) error {
	enqueueCtx, cancel := compensationContext(ctx)
	defer cancel()
	return service.assets.EnqueueProcessing(enqueueCtx, assetID)
}

func compensationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), compensationTimeout)
}

func (service *Service) start(
	ctx context.Context,
	session content.Session,
) (content.Session, error) {
	var err error
	switch session.Status {
	case content.StatusFailed:
		session, err = service.content.Transition(ctx, session.ID, content.StatusPending)
		if err != nil {
			return content.Session{}, fmt.Errorf("reset failed content upload: %w", err)
		}
		fallthrough
	case content.StatusPending:
		session, err = service.content.Transition(ctx, session.ID, content.StatusUploading)
		if err != nil {
			return content.Session{}, fmt.Errorf("start content upload: %w", err)
		}
		return session, nil
	case content.StatusUploading:
		return content.Session{}, ErrInProgress
	case content.StatusCompleted:
		return content.Session{}, ErrInconsistent
	default:
		return content.Session{}, ErrInconsistent
	}
}

func (service *Service) completeExisting(
	ctx context.Context,
	session content.Session,
) error {
	switch session.Status {
	case content.StatusCompleted:
		return nil
	case content.StatusUploading:
		_, err := service.content.Transition(ctx, session.ID, content.StatusCompleted)
		return err
	case content.StatusPending, content.StatusFailed:
		started, err := service.start(ctx, session)
		if err != nil {
			return err
		}
		_, err = service.content.Transition(ctx, started.ID, content.StatusCompleted)
		return err
	default:
		return ErrInconsistent
	}
}

func (service *Service) fail(ctx context.Context, uploadID uuid.UUID) error {
	_, err := service.content.Transition(ctx, uploadID, content.StatusFailed)
	if err != nil {
		return fmt.Errorf("mark content upload failed: %w", err)
	}
	return nil
}

func matchesSession(asset media.Asset, session content.Session) bool {
	return asset.SourceUploadID == session.ID &&
		asset.ContentTrackID == session.ReleaseTrackID &&
		asset.OriginalObjectKey == session.ObjectKey &&
		asset.OriginalFilename == session.OriginalName
}
