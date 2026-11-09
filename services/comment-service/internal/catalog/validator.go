package catalog

import (
	"context"
	"errors"
	"fmt"
	"time"

	catalogv1 "github.com/bevzick/Infinitune/services/comment-service/api/catalog/v1"
	"github.com/bevzick/Infinitune/services/comment-service/internal/comment"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const validationTimeout = 3 * time.Second

type validationClient interface {
	ValidateTrackIds(context.Context, *catalogv1.ValidateTrackIdsRequest, ...grpc.CallOption) (
		*catalogv1.ValidateTrackIdsResponse, error,
	)
}

type TrackValidator struct {
	client  validationClient
	timeout time.Duration
}

func NewTrackValidator(client validationClient) *TrackValidator {
	return &TrackValidator{client: client, timeout: validationTimeout}
}

func (validator *TrackValidator) ValidateTrack(ctx context.Context, trackID uuid.UUID) error {
	callCtx, cancel := context.WithTimeout(ctx, validator.timeout)
	defer cancel()
	response, err := validator.client.ValidateTrackIds(callCtx,
		&catalogv1.ValidateTrackIdsRequest{TrackIds: []string{trackID.String()}})
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errors.Is(callCtx.Err(), context.DeadlineExceeded) || status.Code(err) == codes.DeadlineExceeded {
			return fmt.Errorf("%w: ValidateTrackIds", comment.ErrCatalogTimeout)
		}
		return fmt.Errorf("%w: ValidateTrackIds: %v", comment.ErrCatalogUnavailable, err)
	}
	if len(response.GetMissingTrackIds()) != 0 || len(response.GetValidTrackIds()) != 1 ||
		response.GetValidTrackIds()[0] != trackID.String() {
		return comment.ErrTrackNotFound
	}
	return nil
}

var _ comment.TrackValidator = (*TrackValidator)(nil)
