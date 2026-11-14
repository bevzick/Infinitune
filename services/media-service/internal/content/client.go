package content

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	contentv1 "github.com/bevzick/Infinitune/services/media-service/api/content/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const requestTimeout = 3 * time.Second

var (
	ErrInvalid           = errors.New("invalid content upload session")
	ErrNotFound          = errors.New("content upload session not found")
	ErrForbidden         = errors.New("content upload session forbidden")
	ErrInvalidTransition = errors.New("invalid content upload transition")
	ErrConflict          = errors.New("content upload changed concurrently")
	ErrUnavailable       = errors.New("content service unavailable")
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusUploading Status = "uploading"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

type Session struct {
	ID                uuid.UUID
	ReleaseTrackID    uuid.UUID
	RequestedByUserID uuid.UUID
	ObjectKey         string
	OriginalName      string
	MIMEType          string
	SizeBytes         int64
	Status            Status
}

type grpcClient interface {
	GetUploadSession(context.Context, *contentv1.GetUploadSessionRequest, ...grpc.CallOption) (*contentv1.UploadSession, error)
	TransitionUpload(context.Context, *contentv1.TransitionUploadRequest, ...grpc.CallOption) (*contentv1.UploadSession, error)
	CheckTrackPlayback(context.Context, *contentv1.CheckTrackPlaybackRequest, ...grpc.CallOption) (*contentv1.CheckTrackPlaybackResponse, error)
	CheckTrackImport(context.Context, *contentv1.CheckTrackImportRequest, ...grpc.CallOption) (*contentv1.CheckTrackImportResponse, error)
}

type Client struct{ client grpcClient }

func NewClient(client grpcClient) *Client { return &Client{client: client} }

func (client *Client) CanImport(
	ctx context.Context,
	userID uuid.UUID,
	trackID uuid.UUID,
) error {
	if userID == uuid.Nil || trackID == uuid.Nil {
		return ErrInvalid
	}
	callCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	response, err := client.client.CheckTrackImport(callCtx, &contentv1.CheckTrackImportRequest{
		UserId: userID.String(), ReleaseTrackId: trackID.String(),
	})
	if err != nil {
		return mapError(ctx, err)
	}
	if response == nil || !response.GetAllowed() {
		return ErrForbidden
	}
	return nil
}

func (client *Client) CanStream(
	ctx context.Context,
	trackID uuid.UUID,
) (bool, error) {
	if trackID == uuid.Nil {
		return false, ErrInvalid
	}
	callCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	response, err := client.client.CheckTrackPlayback(
		callCtx, &contentv1.CheckTrackPlaybackRequest{ReleaseTrackId: trackID.String()},
	)
	if err != nil {
		return false, mapError(ctx, err)
	}
	if response == nil {
		return false, fmt.Errorf("%w: empty playback access response", ErrUnavailable)
	}
	return response.GetAllowed(), nil
}

func (client *Client) Session(
	ctx context.Context,
	userID uuid.UUID,
	uploadID uuid.UUID,
) (Session, error) {
	callCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	response, err := client.client.GetUploadSession(callCtx, &contentv1.GetUploadSessionRequest{
		UploadId: uploadID.String(), UserId: userID.String(),
	})
	if err != nil {
		return Session{}, mapError(ctx, err)
	}
	return parseSession(response)
}

func (client *Client) Transition(
	ctx context.Context,
	uploadID uuid.UUID,
	target Status,
) (Session, error) {
	callCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	response, err := client.client.TransitionUpload(callCtx, &contentv1.TransitionUploadRequest{
		UploadId: uploadID.String(), TargetStatus: string(target),
	})
	if err != nil {
		return Session{}, mapError(ctx, err)
	}
	return parseSession(response)
}

func parseSession(response *contentv1.UploadSession) (Session, error) {
	if response == nil {
		return Session{}, fmt.Errorf("%w: empty response", ErrUnavailable)
	}
	id, err := uuid.Parse(response.GetId())
	if err != nil || id == uuid.Nil {
		return Session{}, fmt.Errorf("%w: invalid upload id", ErrUnavailable)
	}
	trackID, err := uuid.Parse(response.GetReleaseTrackId())
	if err != nil || trackID == uuid.Nil {
		return Session{}, fmt.Errorf("%w: invalid release track id", ErrUnavailable)
	}
	userID, err := uuid.Parse(response.GetRequestedByUserId())
	if err != nil || userID == uuid.Nil || strings.TrimSpace(response.GetObjectKey()) == "" ||
		strings.TrimSpace(response.GetOriginalName()) == "" ||
		strings.TrimSpace(response.GetMimeType()) == "" || response.GetSizeBytes() < 1 {
		return Session{}, fmt.Errorf("%w: malformed upload session", ErrUnavailable)
	}
	parsed := Session{
		ID: id, ReleaseTrackID: trackID, RequestedByUserID: userID,
		ObjectKey: response.GetObjectKey(), OriginalName: response.GetOriginalName(),
		MIMEType: response.GetMimeType(), SizeBytes: response.GetSizeBytes(),
		Status: Status(response.GetStatus()),
	}
	switch parsed.Status {
	case StatusPending, StatusUploading, StatusCompleted, StatusFailed:
		return parsed, nil
	default:
		return Session{}, fmt.Errorf("%w: invalid upload status", ErrUnavailable)
	}
}

func mapError(parent context.Context, err error) error {
	if parent.Err() != nil {
		return parent.Err()
	}
	switch status.Code(err) {
	case codes.InvalidArgument:
		return ErrInvalid
	case codes.NotFound:
		return ErrNotFound
	case codes.PermissionDenied:
		return ErrForbidden
	case codes.FailedPrecondition:
		return ErrInvalidTransition
	case codes.Aborted:
		return ErrConflict
	case codes.DeadlineExceeded, codes.Unavailable:
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	default:
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
}
