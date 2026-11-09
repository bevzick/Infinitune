package catalog

import (
	"context"
	"errors"
	"fmt"
	"time"

	catalogv1 "github.com/bevzick/Infinitune/services/interaction-service/api/catalog/v1"
	"github.com/bevzick/Infinitune/services/interaction-service/internal/interaction"
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
	ValidateAlbumIds(context.Context, *catalogv1.ValidateAlbumIdsRequest, ...grpc.CallOption) (
		*catalogv1.ValidateAlbumIdsResponse, error,
	)
}

type Validator struct {
	client  validationClient
	timeout time.Duration
}

func NewValidator(client validationClient) *Validator {
	return &Validator{client: client, timeout: validationTimeout}
}

func (validator *Validator) ValidateTrack(ctx context.Context, trackID uuid.UUID) error {
	callCtx, cancel := context.WithTimeout(ctx, validator.timeout)
	defer cancel()
	response, err := validator.client.ValidateTrackIds(callCtx,
		&catalogv1.ValidateTrackIdsRequest{TrackIds: []string{trackID.String()}})
	if err != nil {
		return validationError(ctx, callCtx, err, "ValidateTrackIds")
	}
	if len(response.GetMissingTrackIds()) != 0 || len(response.GetValidTrackIds()) != 1 ||
		response.GetValidTrackIds()[0] != trackID.String() {
		return interaction.ErrTrackNotFound
	}
	return nil
}

func (validator *Validator) ValidateAlbum(ctx context.Context, albumID uuid.UUID) error {
	callCtx, cancel := context.WithTimeout(ctx, validator.timeout)
	defer cancel()
	response, err := validator.client.ValidateAlbumIds(callCtx,
		&catalogv1.ValidateAlbumIdsRequest{AlbumIds: []string{albumID.String()}})
	if err != nil {
		return validationError(ctx, callCtx, err, "ValidateAlbumIds")
	}
	if len(response.GetMissingAlbumIds()) != 0 || len(response.GetValidAlbumIds()) != 1 ||
		response.GetValidAlbumIds()[0] != albumID.String() {
		return interaction.ErrAlbumNotFound
	}
	return nil
}

func validationError(parent, call context.Context, err error, operation string) error {
	if parent.Err() != nil {
		return parent.Err()
	}
	if errors.Is(call.Err(), context.DeadlineExceeded) || status.Code(err) == codes.DeadlineExceeded {
		return fmt.Errorf("%w: %s", interaction.ErrCatalogTimeout, operation)
	}
	return fmt.Errorf("%w: %s: %v", interaction.ErrCatalogUnavailable, operation, err)
}

var _ interaction.CatalogValidator = (*Validator)(nil)
