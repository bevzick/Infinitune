package grpcapi

import (
	"context"
	"errors"
	"time"

	commentv1 "github.com/bevzick/Infinitune/services/comment-service/api/comment/v1"
	"github.com/bevzick/Infinitune/services/comment-service/internal/comment"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const lookupTimeout = 3 * time.Second

type commentLookup interface {
	ValidateIDs(context.Context, []uuid.UUID) (comment.IDValidation, error)
}

type CommentServer struct {
	commentv1.UnimplementedCommentServiceServer
	comments commentLookup
	timeout  time.Duration
}

func NewCommentServer(comments commentLookup) *CommentServer {
	return &CommentServer{comments: comments, timeout: lookupTimeout}
}

func (server *CommentServer) ValidateCommentIds(
	ctx context.Context,
	request *commentv1.ValidateCommentIdsRequest,
) (*commentv1.ValidateCommentIdsResponse, error) {
	ids, err := parseCommentIDs(request.GetCommentIds())
	if err != nil {
		return nil, err
	}
	callCtx, cancel := context.WithTimeout(ctx, server.timeout)
	defer cancel()
	result, err := server.comments.ValidateIDs(callCtx, ids)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(callCtx.Err(), context.DeadlineExceeded) {
			return nil, status.Error(codes.DeadlineExceeded, "comment lookup timed out")
		}
		if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			return nil, status.Error(codes.Canceled, "comment lookup canceled")
		}
		return nil, status.Error(codes.Internal, "comment lookup failed")
	}
	return &commentv1.ValidateCommentIdsResponse{
		ValidCommentIds: uuidStrings(result.ValidIDs), MissingCommentIds: uuidStrings(result.MissingIDs),
	}, nil
}

func parseCommentIDs(values []string) ([]uuid.UUID, error) {
	if len(values) == 0 {
		return nil, status.Error(codes.InvalidArgument, "comment_ids must not be empty")
	}
	if len(values) > 100 {
		return nil, status.Error(codes.InvalidArgument, "comment_ids must not contain more than 100 ids")
	}
	ids := make([]uuid.UUID, 0, len(values))
	seen := make(map[uuid.UUID]struct{}, len(values))
	for _, value := range values {
		id, err := uuid.Parse(value)
		if err != nil || id == uuid.Nil {
			return nil, status.Error(codes.InvalidArgument, "comment_ids must contain UUIDs")
		}
		if _, exists := seen[id]; exists {
			return nil, status.Error(codes.InvalidArgument, "comment_ids must not contain duplicates")
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, nil
}

func uuidStrings(ids []uuid.UUID) []string {
	result := make([]string, len(ids))
	for index, id := range ids {
		result[index] = id.String()
	}
	return result
}
