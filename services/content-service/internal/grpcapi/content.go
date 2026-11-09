package grpcapi

import (
	"context"
	"errors"

	contentv1 "github.com/bevzick/Infinitune/services/content-service/api/content/v1"
	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AccessChecker interface {
	Check(context.Context, uuid.UUID, uuid.UUID, creator.Permission) (creator.AccessDecision, error)
}

type ContentServer struct {
	contentv1.UnimplementedContentServiceServer
	access AccessChecker
}

func NewContentServer(access AccessChecker) *ContentServer {
	return &ContentServer{access: access}
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
