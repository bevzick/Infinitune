package catalog

import (
	"context"
	"errors"
	"testing"

	catalogv1 "github.com/bevzick/Infinitune/services/playlist-service/api/catalog/v1"
	"github.com/bevzick/Infinitune/services/playlist-service/internal/playlist"
	"github.com/google/uuid"
	"google.golang.org/grpc"
)

func TestTrackValidatorUsesCatalogValidation(t *testing.T) {
	trackID := uuid.New()
	client := &validationClientStub{response: &catalogv1.ValidateTrackIdsResponse{
		ValidTrackIds: []string{trackID.String()},
	}}
	validator := NewTrackValidator(client)

	if err := validator.ValidateTrack(context.Background(), trackID); err != nil {
		t.Fatalf("ValidateTrack() error = %v", err)
	}
	if len(client.request.GetTrackIds()) != 1 || client.request.GetTrackIds()[0] != trackID.String() {
		t.Fatalf("ValidateTrackIds request = %+v", client.request)
	}
}

func TestTrackValidatorRejectsMissingOrUnexpectedResponse(t *testing.T) {
	trackID := uuid.New()
	tests := []struct {
		name     string
		response *catalogv1.ValidateTrackIdsResponse
	}{
		{name: "missing", response: &catalogv1.ValidateTrackIdsResponse{
			MissingTrackIds: []string{trackID.String()},
		}},
		{name: "unexpected valid id", response: &catalogv1.ValidateTrackIdsResponse{
			ValidTrackIds: []string{uuid.New().String()},
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			validator := NewTrackValidator(&validationClientStub{response: test.response})
			if err := validator.ValidateTrack(context.Background(), trackID); !errors.Is(err, playlist.ErrTrackNotFound) {
				t.Fatalf("ValidateTrack() error = %v, want %v", err, playlist.ErrTrackNotFound)
			}
		})
	}
}

func TestTrackValidatorPropagatesTransportError(t *testing.T) {
	want := errors.New("transport failure")
	validator := NewTrackValidator(&validationClientStub{err: want})
	if err := validator.ValidateTrack(context.Background(), uuid.New()); !errors.Is(err, playlist.ErrCatalogUnavailable) {
		t.Fatalf("ValidateTrack() error = %v, want %v", err, playlist.ErrCatalogUnavailable)
	}
}

type validationClientStub struct {
	request  *catalogv1.ValidateTrackIdsRequest
	response *catalogv1.ValidateTrackIdsResponse
	err      error
}

func (stub *validationClientStub) ValidateTrackIds(
	_ context.Context,
	request *catalogv1.ValidateTrackIdsRequest,
	_ ...grpc.CallOption,
) (*catalogv1.ValidateTrackIdsResponse, error) {
	stub.request = request
	return stub.response, stub.err
}
