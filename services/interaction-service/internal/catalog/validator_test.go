package catalog

import (
	"context"
	"errors"
	"testing"

	catalogv1 "github.com/bevzick/Infinitune/services/interaction-service/api/catalog/v1"
	"github.com/bevzick/Infinitune/services/interaction-service/internal/interaction"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestValidatorAcceptsAvailableTrackAndAlbum(t *testing.T) {
	trackID, albumID := uuid.New(), uuid.New()
	validator := NewValidator(&validationClientStub{trackID: trackID, albumID: albumID})
	if err := validator.ValidateTrack(context.Background(), trackID); err != nil {
		t.Fatalf("ValidateTrack() error = %v", err)
	}
	if err := validator.ValidateAlbum(context.Background(), albumID); err != nil {
		t.Fatalf("ValidateAlbum() error = %v", err)
	}
}

func TestValidatorMapsMissingAndUnavailableTargets(t *testing.T) {
	trackID, albumID := uuid.New(), uuid.New()
	missing := NewValidator(&validationClientStub{})
	if err := missing.ValidateTrack(context.Background(), trackID); !errors.Is(err, interaction.ErrTrackNotFound) {
		t.Fatalf("ValidateTrack() error = %v", err)
	}
	if err := missing.ValidateAlbum(context.Background(), albumID); !errors.Is(err, interaction.ErrAlbumNotFound) {
		t.Fatalf("ValidateAlbum() error = %v", err)
	}
	timedOut := NewValidator(&validationClientStub{err: status.Error(codes.DeadlineExceeded, "timeout")})
	if err := timedOut.ValidateTrack(context.Background(), trackID); !errors.Is(err, interaction.ErrCatalogTimeout) {
		t.Fatalf("ValidateTrack(timeout) error = %v", err)
	}
	unavailable := NewValidator(&validationClientStub{err: status.Error(codes.Unavailable, "offline")})
	if err := unavailable.ValidateAlbum(context.Background(), albumID); !errors.Is(err, interaction.ErrCatalogUnavailable) {
		t.Fatalf("ValidateAlbum(unavailable) error = %v", err)
	}
}

type validationClientStub struct {
	trackID uuid.UUID
	albumID uuid.UUID
	err     error
}

func (stub *validationClientStub) ValidateTrackIds(
	context.Context, *catalogv1.ValidateTrackIdsRequest, ...grpc.CallOption,
) (*catalogv1.ValidateTrackIdsResponse, error) {
	if stub.err != nil {
		return nil, stub.err
	}
	if stub.trackID == uuid.Nil {
		return &catalogv1.ValidateTrackIdsResponse{MissingTrackIds: []string{uuid.NewString()}}, nil
	}
	return &catalogv1.ValidateTrackIdsResponse{ValidTrackIds: []string{stub.trackID.String()}}, nil
}

func (stub *validationClientStub) ValidateAlbumIds(
	context.Context, *catalogv1.ValidateAlbumIdsRequest, ...grpc.CallOption,
) (*catalogv1.ValidateAlbumIdsResponse, error) {
	if stub.err != nil {
		return nil, stub.err
	}
	if stub.albumID == uuid.Nil {
		return &catalogv1.ValidateAlbumIdsResponse{MissingAlbumIds: []string{uuid.NewString()}}, nil
	}
	return &catalogv1.ValidateAlbumIdsResponse{ValidAlbumIds: []string{stub.albumID.String()}}, nil
}
