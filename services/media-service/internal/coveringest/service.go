package coveringest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"strings"
	"time"

	"github.com/bevzick/Infinitune/services/media-service/internal/content"
	"github.com/google/uuid"
)

const MaximumCoverBytes int64 = 10 * 1024 * 1024
const cleanupTimeout = 5 * time.Second

var (
	ErrInvalid      = errors.New("invalid cover upload")
	ErrTooLarge     = errors.New("cover image exceeds 10 MiB")
	ErrInProgress   = errors.New("cover upload already in progress")
	ErrInvalidState = errors.New("cover upload session state is invalid")
	ErrStorage      = errors.New("cover object storage failed")
)

type CoverSessions interface {
	Session(context.Context, uuid.UUID, uuid.UUID) (content.CoverSession, error)
	Transition(context.Context, uuid.UUID, content.Status) (content.CoverSession, error)
	Complete(context.Context, uuid.UUID) (content.CoverSession, error)
}
type ObjectStore interface {
	PutIfAbsent(context.Context, string, io.Reader, int64, string) error
	Delete(context.Context, string) error
}

type Service struct {
	sessions CoverSessions
	objects  ObjectStore
}

func NewService(sessions CoverSessions, objects ObjectStore) *Service {
	return &Service{sessions: sessions, objects: objects}
}

type Result struct {
	ReleaseID uuid.UUID
	Status    content.Status
}

func (s *Service) Ingest(ctx context.Context, actorID, uploadID uuid.UUID, mimeType string, length int64, body io.Reader) (Result, bool, error) {
	if actorID == uuid.Nil {
		return Result{}, false, content.ErrForbidden
	}
	if uploadID == uuid.Nil || body == nil {
		return Result{}, false, ErrInvalid
	}
	session, err := s.sessions.Session(ctx, actorID, uploadID)
	if err != nil {
		return Result{}, false, fmt.Errorf("load cover upload: %w", err)
	}
	if session.ID != uploadID || session.RequestedByUserID != actorID {
		return Result{}, false, content.ErrForbidden
	}
	if !validObjectKey(session) {
		return Result{}, false, ErrInvalidState
	}
	if session.Status == content.StatusCompleted {
		return Result{ReleaseID: session.ReleaseID, Status: session.Status}, false, nil
	}
	if session.Status == content.StatusUploading {
		return Result{}, false, ErrInProgress
	}
	// Failed sessions use a new session/key rather than overwrite an existing object.
	if session.Status != content.StatusPending {
		return Result{}, false, ErrInvalidState
	}
	if length < 1 || length > MaximumCoverBytes {
		return Result{}, false, ErrTooLarge
	}
	contentType, params, parseErr := mime.ParseMediaType(strings.TrimSpace(mimeType))
	if parseErr != nil || len(params) > 0 || !strings.EqualFold(contentType, session.MIMEType) || length != session.SizeBytes {
		return Result{}, false, ErrInvalid
	}
	imageBytes, err := io.ReadAll(io.LimitReader(body, MaximumCoverBytes+1))
	if err != nil {
		return Result{}, false, fmt.Errorf("read uploaded cover: %w", err)
	}
	if int64(len(imageBytes)) > MaximumCoverBytes {
		return Result{}, false, ErrTooLarge
	}
	if int64(len(imageBytes)) != session.SizeBytes {
		return Result{}, false, ErrInvalid
	}
	if err = validateImage(imageBytes, session.MIMEType); err != nil {
		return Result{}, false, err
	}

	started, err := s.sessions.Transition(ctx, session.ID, content.StatusUploading)
	if err != nil {
		return Result{}, false, fmt.Errorf("lock cover upload: %w", err)
	}
	if started.Status != content.StatusUploading || started.ObjectKey != session.ObjectKey {
		return Result{}, false, ErrInvalidState
	}
	if err = s.objects.PutIfAbsent(ctx, session.ObjectKey, bytes.NewReader(imageBytes), int64(len(imageBytes)), session.MIMEType); err != nil {
		// A failed conditional PUT may mean an existing object already owns
		// this key. Never delete it blindly: it may belong to an earlier
		// successful request whose response was lost.
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
		defer cancel()
		_, transitionErr := s.sessions.Transition(cleanupCtx, session.ID, content.StatusFailed)
		return Result{}, false, errors.Join(fmt.Errorf("store cover: %w: %v", ErrStorage, err), transitionErr)
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
	defer cancel()
	completed, err := s.sessions.Complete(cleanupCtx, session.ID)
	if err != nil {
		// If the Content RPC timed out, do not delete a potentially committed
		// cover. An operator can reconcile the uploading session later.
		if errors.Is(err, content.ErrInvalidTransition) || errors.Is(err, content.ErrConflict) {
			deleteErr := s.objects.Delete(cleanupCtx, session.ObjectKey)
			_, failErr := s.sessions.Transition(cleanupCtx, session.ID, content.StatusFailed)
			return Result{}, false, errors.Join(err, deleteErr, failErr)
		}
		return Result{}, false, fmt.Errorf("complete cover upload: %w", err)
	}
	if completed.Status != content.StatusCompleted || completed.ID != session.ID {
		return Result{}, false, ErrInvalidState
	}
	return Result{ReleaseID: completed.ReleaseID, Status: completed.Status}, true, nil
}

func validObjectKey(s content.CoverSession) bool {
	if s.ID == uuid.Nil || s.ReleaseID == uuid.Nil || s.RequestedByUserID == uuid.Nil {
		return false
	}
	// The private object key is generated by Content, never by the browser.
	// Refuse other prefixes to prevent accidental writes to unrelated objects.
	parts := strings.Split(s.ObjectKey, "/")
	if len(parts) != 6 || parts[0] != "creator-content" || parts[2] != s.ReleaseID.String() ||
		parts[3] != "covers" || parts[4] != s.ID.String() {
		return false
	}
	creatorID, err := uuid.Parse(parts[1])
	if err != nil || creatorID == uuid.Nil {
		return false
	}
	if parts[5] != "cover.jpg" && parts[5] != "cover.jpeg" && parts[5] != "cover.png" && parts[5] != "cover.webp" {
		return false
	}
	if s.MIMEType == "image/png" {
		return parts[5] == "cover.png"
	}
	if s.MIMEType == "image/webp" {
		return parts[5] == "cover.webp"
	}
	if s.MIMEType == "image/jpeg" {
		return parts[5] == "cover.jpg" || parts[5] == "cover.jpeg"
	}
	return false
}
