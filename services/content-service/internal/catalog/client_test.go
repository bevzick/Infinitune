package catalog

import (
	"context"
	"errors"
	"testing"
	"time"

	catalogv1 "github.com/bevzick/Infinitune/services/content-service/api/catalog/v1"
	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestClientPublishesRelease(t *testing.T) {
	releaseID, artistID, albumID := uuid.New(), uuid.New(), uuid.New()
	contentTrackID, catalogTrackID, mediaID := uuid.New(), uuid.New(), uuid.New()
	now := time.Now().UTC()
	stub := &catalogClientStub{response: &catalogv1.PublishReleaseResponse{
		ArtistId: artistID.String(), AlbumId: albumID.String(),
		Tracks: []*catalogv1.PublishedTrack{{
			ContentTrackId: contentTrackID.String(), CatalogTrackId: catalogTrackID.String(),
		}}, PublishedAt: timestamppb.New(now),
	}}
	result, err := NewClient(stub).Publish(context.Background(), PublicationInput{
		SourceReleaseID: releaseID, ArtistID: artistID, ArtistName: "Artist",
		AlbumID: &albumID, Title: "Album", Type: release.TypeAlbum,
		ReleaseDate: time.Date(2027, 3, 4, 0, 0, 0, 0, time.UTC),
		Tracks: []TrackInput{{
			ID: catalogTrackID, ContentTrackID: contentTrackID, Title: "Track",
			DurationMS: 123000, TrackNumber: 1, DiscNumber: 1, MediaAssetID: mediaID,
		}},
	})
	if err != nil || result.ArtistID != artistID || result.AlbumID == nil ||
		*result.AlbumID != albumID || len(result.Tracks) != 1 ||
		!result.PublishedAt.Equal(now) {
		t.Fatalf("Publish() = %+v, %v", result, err)
	}
	if stub.request.GetSourceReleaseId() != releaseID.String() ||
		stub.request.GetAlbum().GetAlbumType() != "album" ||
		stub.request.GetTracks()[0].GetDurationMs() != 123000 {
		t.Fatalf("request = %+v", stub.request)
	}
}

func TestClientMapsPublicationErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{
		{name: "conflict", err: status.Error(codes.AlreadyExists, "exists"), want: ErrConflict},
		{name: "unavailable", err: status.Error(codes.Unavailable, "down"), want: ErrUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewClient(&catalogClientStub{err: test.err}).Publish(
				context.Background(), PublicationInput{},
			)
			if !errors.Is(err, test.want) {
				t.Fatalf("Publish() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestClientRejectsMalformedPublication(t *testing.T) {
	_, err := NewClient(&catalogClientStub{response: &catalogv1.PublishReleaseResponse{}}).
		Publish(context.Background(), PublicationInput{})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("Publish() error = %v", err)
	}
}

type catalogClientStub struct {
	request  *catalogv1.PublishReleaseRequest
	response *catalogv1.PublishReleaseResponse
	err      error
}

func (stub *catalogClientStub) PublishRelease(
	_ context.Context,
	request *catalogv1.PublishReleaseRequest,
	_ ...grpc.CallOption,
) (*catalogv1.PublishReleaseResponse, error) {
	stub.request = request
	return stub.response, stub.err
}

var _ catalogv1.CatalogServiceClient = (*catalogClientStub)(nil)
