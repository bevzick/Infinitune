package catalog

import (
	"context"
	"errors"
	"testing"

	catalogv1 "github.com/bevzick/Infinitune/services/history-service/api/catalog/v1"
	"github.com/bevzick/Infinitune/services/history-service/internal/history"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestDurationUsesCatalogMetadata(t *testing.T) {
	trackID := uuid.New()
	client := NewClient(trackClientStub{response: &catalogv1.GetTracksResponse{Tracks: []*catalogv1.Track{{Id: trackID.String(), DurationMs: 123000}}}})
	duration, err := client.Duration(context.Background(), trackID)
	if err != nil || duration != 123000 {
		t.Fatalf("Duration() = %d, %v", duration, err)
	}
}

func TestDurationMapsCatalogErrors(t *testing.T) {
	trackID := uuid.New()
	for _, test := range []struct {
		name      string
		err, want error
	}{
		{"missing", status.Error(codes.NotFound, "missing"), history.ErrTrackNotFound},
		{"timeout", status.Error(codes.DeadlineExceeded, "timeout"), history.ErrCatalogTimeout},
		{"unavailable", status.Error(codes.Unavailable, "offline"), history.ErrCatalogUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewClient(trackClientStub{err: test.err}).Duration(context.Background(), trackID)
			if !errors.Is(err, test.want) {
				t.Fatalf("Duration() error = %v, want %v", err, test.want)
			}
		})
	}
}

type trackClientStub struct {
	response *catalogv1.GetTracksResponse
	err      error
}

func (stub trackClientStub) GetTracks(context.Context, *catalogv1.GetTracksRequest, ...grpc.CallOption) (*catalogv1.GetTracksResponse, error) {
	return stub.response, stub.err
}
