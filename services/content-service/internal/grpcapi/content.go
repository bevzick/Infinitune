package grpcapi

import (
	"context"
	"errors"

	contentv1 "github.com/bevzick/Infinitune/services/content-service/api/content/v1"
	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/bevzick/Infinitune/services/content-service/internal/playback"
	"github.com/bevzick/Infinitune/services/content-service/internal/upload"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AccessChecker interface {
	Check(context.Context, uuid.UUID, uuid.UUID, creator.Permission) (creator.AccessDecision, error)
}

type UploadSessions interface {
	SessionForUser(context.Context, uuid.UUID, uuid.UUID) (upload.Session, error)
	Transition(context.Context, uuid.UUID, upload.Status) (upload.Session, error)
}

type PlaybackAccess interface {
	IsTrackPlayable(context.Context, uuid.UUID) (bool, error)
}

type ContentServer struct {
	contentv1.UnimplementedContentServiceServer
	access   AccessChecker
	uploads  UploadSessions
	playback PlaybackAccess
}

func NewContentServer(
	access AccessChecker,
	uploads UploadSessions,
	playback PlaybackAccess,
) *ContentServer {
	return &ContentServer{access: access, uploads: uploads, playback: playback}
}

func (server *ContentServer) CheckTrackPlayback(
	ctx context.Context,
	request *contentv1.CheckTrackPlaybackRequest,
) (*contentv1.CheckTrackPlaybackResponse, error) {
	if request == nil || server.playback == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	trackID, err := parseUUID(request.GetReleaseTrackId(), "release_track_id")
	if err != nil {
		return nil, err
	}
	allowed, err := server.playback.IsTrackPlayable(ctx, trackID)
	if err != nil {
		if errors.Is(err, playback.ErrInvalid) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, "playback access could not be checked")
	}
	return &contentv1.CheckTrackPlaybackResponse{Allowed: allowed}, nil
}

func (server *ContentServer) GetUploadSession(
	ctx context.Context,
	request *contentv1.GetUploadSessionRequest,
) (*contentv1.UploadSession, error) {
	if request == nil || server.uploads == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	uploadID, err := parseUUID(request.GetUploadId(), "upload_id")
	if err != nil {
		return nil, err
	}
	userID, err := parseUUID(request.GetUserId(), "user_id")
	if err != nil {
		return nil, err
	}
	session, err := server.uploads.SessionForUser(ctx, userID, uploadID)
	if err != nil {
		return nil, mapUploadError(err)
	}
	return uploadResponse(session), nil
}

func (server *ContentServer) TransitionUpload(
	ctx context.Context,
	request *contentv1.TransitionUploadRequest,
) (*contentv1.UploadSession, error) {
	if request == nil || server.uploads == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	uploadID, err := parseUUID(request.GetUploadId(), "upload_id")
	if err != nil {
		return nil, err
	}
	session, err := server.uploads.Transition(
		ctx, uploadID, upload.Status(request.GetTargetStatus()),
	)
	if err != nil {
		return nil, mapUploadError(err)
	}
	return uploadResponse(session), nil
}

func parseUUID(value, field string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, status.Errorf(codes.InvalidArgument, "%s must be a UUID", field)
	}
	return id, nil
}

func mapUploadError(err error) error {
	switch {
	case errors.Is(err, creator.ErrForbidden):
		return status.Error(codes.PermissionDenied, "upload session is not available to this user")
	case errors.Is(err, upload.ErrInvalid):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, upload.ErrNotFound):
		return status.Error(codes.NotFound, "upload session was not found")
	case errors.Is(err, upload.ErrInvalidTransition), errors.Is(err, upload.ErrReleaseLocked):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, upload.ErrConflict):
		return status.Error(codes.Aborted, "upload session changed concurrently")
	default:
		return status.Error(codes.Internal, "upload session operation failed")
	}
}

func uploadResponse(session upload.Session) *contentv1.UploadSession {
	return &contentv1.UploadSession{
		Id: session.ID.String(), ReleaseTrackId: session.ReleaseTrackID.String(),
		RequestedByUserId: session.RequestedByUserID.String(), ObjectKey: session.ObjectKey,
		OriginalName: session.OriginalName, MimeType: session.MIMEType,
		SizeBytes: session.SizeBytes, Status: string(session.Status),
	}
}

func (server *ContentServer) CheckPermission(
	ctx context.Context,
	request *contentv1.CheckPermissionRequest,
) (*contentv1.CheckPermissionResponse, error) {
	if request == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	creatorID, err := uuid.Parse(request.GetCreatorId())
	if err != nil || creatorID == uuid.Nil {
		return nil, status.Error(codes.InvalidArgument, "creator_id must be a UUID")
	}
	userID, err := uuid.Parse(request.GetUserId())
	if err != nil || userID == uuid.Nil {
		return nil, status.Error(codes.InvalidArgument, "user_id must be a UUID")
	}
	decision, err := server.access.Check(ctx, userID, creatorID, creator.Permission(request.GetPermission()))
	if err != nil {
		switch {
		case errors.Is(err, creator.ErrInvalid):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case errors.Is(err, creator.ErrNotFound):
			return nil, status.Error(codes.NotFound, "creator account was not found")
		default:
			return nil, status.Error(codes.Internal, "permission could not be checked")
		}
	}
	return &contentv1.CheckPermissionResponse{
		Allowed: decision.Allowed, MemberRole: string(decision.Role),
		VerificationStatus: string(decision.VerificationStatus),
	}, nil
}

var _ contentv1.ContentServiceServer = (*ContentServer)(nil)
