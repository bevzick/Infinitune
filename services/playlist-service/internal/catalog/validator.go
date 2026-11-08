package catalog

import (
	"context"
	"fmt"
	"time"

	catalogv1 "github.com/bevzick/Infinitune/services/playlist-service/api/catalog/v1"
	"github.com/bevzick/Infinitune/services/playlist-service/internal/playlist"
	"github.com/google/uuid"
	"google.golang.org/grpc"
)

const validationTimeout = 3 * time.Second

type validationClient interface {
	ValidateTrackIds(
		context.Context,
		*catalogv1.ValidateTrackIdsRequest,
		...grpc.CallOption,
	) (*catalogv1.ValidateTrackIdsResponse, error)
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
	response, err := validator.client.ValidateTrackIds(
		callCtx,
		&catalogv1.ValidateTrackIdsRequest{TrackIds: []string{trackID.String()}},
	)
	if err != nil {
		return fmt.Errorf("catalog ValidateTrackIds: %w", err)
	}
	if len(response.GetMissingTrackIds()) != 0 ||
		len(response.GetValidTrackIds()) != 1 || response.GetValidTrackIds()[0] != trackID.String() {
		return playlist.ErrTrackNotFound
	}
	return nil
}

var _ playlist.TrackValidator = (*TrackValidator)(nil)
