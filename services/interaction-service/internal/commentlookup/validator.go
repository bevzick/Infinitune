package commentlookup

import (
	"context"
	"errors"
	"fmt"
	"time"

	commentv1 "github.com/bevzick/Infinitune/services/interaction-service/api/comment/v1"
	"github.com/bevzick/Infinitune/services/interaction-service/internal/interaction"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const validationTimeout = 3 * time.Second

type validationClient interface {
	ValidateCommentIds(context.Context, *commentv1.ValidateCommentIdsRequest, ...grpc.CallOption) (
		*commentv1.ValidateCommentIdsResponse, error,
	)
}

type Validator struct {
	client  validationClient
	timeout time.Duration
}

func NewValidator(client validationClient) *Validator {
	return &Validator{client: client, timeout: validationTimeout}
}

func (validator *Validator) ValidateComment(ctx context.Context, commentID uuid.UUID) error {
	valid, err := validator.ValidateComments(ctx, []uuid.UUID{commentID})
	if err != nil {
		return err
	}
	if _, ok := valid[commentID]; !ok {
		return interaction.ErrCommentNotFound
	}
	return nil
}

func (validator *Validator) ValidateComments(
	ctx context.Context,
	commentIDs []uuid.UUID,
) (map[uuid.UUID]struct{}, error) {
	if len(commentIDs) == 0 {
		return map[uuid.UUID]struct{}{}, nil
	}
	callCtx, cancel := context.WithTimeout(ctx, validator.timeout)
	defer cancel()
	values := make([]string, len(commentIDs))
	requested := make(map[uuid.UUID]struct{}, len(commentIDs))
	for index, id := range commentIDs {
		values[index] = id.String()
		requested[id] = struct{}{}
	}
	response, err := validator.client.ValidateCommentIds(callCtx,
		&commentv1.ValidateCommentIdsRequest{CommentIds: values})
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if errors.Is(callCtx.Err(), context.DeadlineExceeded) || status.Code(err) == codes.DeadlineExceeded {
			return nil, fmt.Errorf("%w: ValidateCommentIds", interaction.ErrCommentTimeout)
		}
		return nil, fmt.Errorf("%w: ValidateCommentIds: %v", interaction.ErrCommentUnavailable, err)
	}
	valid := make(map[uuid.UUID]struct{}, len(response.GetValidCommentIds()))
	seen := make(map[uuid.UUID]struct{}, len(commentIDs))
	for _, value := range append(response.GetValidCommentIds(), response.GetMissingCommentIds()...) {
		id, parseErr := uuid.Parse(value)
		if _, ok := requested[id]; parseErr != nil || id == uuid.Nil || !ok {
			return nil, interaction.ErrCommentUnavailable
		}
		if _, duplicate := seen[id]; duplicate {
			return nil, interaction.ErrCommentUnavailable
		}
		seen[id] = struct{}{}
	}
	if len(seen) != len(requested) {
		return nil, interaction.ErrCommentUnavailable
	}
	for _, value := range response.GetValidCommentIds() {
		id, _ := uuid.Parse(value)
		valid[id] = struct{}{}
	}
	return valid, nil
}

var _ interaction.CommentValidator = (*Validator)(nil)
