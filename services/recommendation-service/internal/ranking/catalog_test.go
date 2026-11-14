package ranking

import (
	"context"
	"testing"

	catalogv1 "github.com/bevzick/Infinitune/services/recommendation-service/api/catalog/v1"
	preferencesignal "github.com/bevzick/Infinitune/services/recommendation-service/internal/signal"
	"github.com/google/uuid"
	"google.golang.org/grpc"
)

func TestCatalogSourceDeduplicatesAndFiltersUnavailableContent(t *testing.T) {
	trackID, explicitID, missingID := uuid.New(), uuid.New(), uuid.New()
	contentID, explicitContentID := uuid.New(), uuid.New()
	client := &catalogStub{
		filtered: []*catalogv1.Track{{Id: trackID.String(), ContentId: contentID.String()}, {Id: explicitID.String(), ContentId: explicitContentID.String(), Explicit: true}},
		validIDs: []string{trackID.String()},
		direct:   []*catalogv1.Track{{Id: trackID.String(), ContentId: contentID.String()}},
	}
	source := NewCatalogSource(client)
	result, err := source.Discover(context.Background(), preferencesignal.Set{
		Preferences:  preferencesignal.Preferences{Enabled: true},
		Interactions: preferencesignal.Interactions{LikedTrackIDs: []uuid.UUID{trackID, missingID}},
	})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(result) != 1 || result[0].TrackID != trackID {
		t.Fatalf("Discover() = %+v", result)
	}
	if len(client.filterRequests) != 1 || client.filterRequests[0].Explicit == nil || client.filterRequests[0].GetExplicit() {
		t.Fatalf("explicit filter = %+v", client.filterRequests[0].Explicit)
	}
}

func TestCatalogSourceLoadsOnlyRequestedCandidatesInOrder(t *testing.T) {
	first, second := uuid.New(), uuid.New()
	client := &catalogStub{direct: []*catalogv1.Track{{Id: first.String(), ContentId: uuid.NewString()}, {Id: second.String(), ContentId: uuid.NewString()}}}
	result, err := NewCatalogSource(client).Candidates(context.Background(), []uuid.UUID{first, second})
	if err != nil {
		t.Fatalf("Candidates() error = %v", err)
	}
	if len(result) != 2 || result[0].TrackID != first || result[0].DiscoveryRank != 0 || result[1].TrackID != second || result[1].DiscoveryRank != 1 {
		t.Fatalf("Candidates() = %+v", result)
	}
}

type catalogStub struct {
	filtered       []*catalogv1.Track
	direct         []*catalogv1.Track
	validIDs       []string
	filterRequests []*catalogv1.FilterTracksRequest
}

func (stub *catalogStub) FilterTracks(_ context.Context, request *catalogv1.FilterTracksRequest, _ ...grpc.CallOption) (*catalogv1.FilterTracksResponse, error) {
	stub.filterRequests = append(stub.filterRequests, request)
	return &catalogv1.FilterTracksResponse{Tracks: stub.filtered}, nil
}
func (stub *catalogStub) ValidateTrackIds(context.Context, *catalogv1.ValidateTrackIdsRequest, ...grpc.CallOption) (*catalogv1.ValidateTrackIdsResponse, error) {
	return &catalogv1.ValidateTrackIdsResponse{ValidTrackIds: stub.validIDs}, nil
}
func (stub *catalogStub) GetTracks(context.Context, *catalogv1.GetTracksRequest, ...grpc.CallOption) (*catalogv1.GetTracksResponse, error) {
	return &catalogv1.GetTracksResponse{Tracks: stub.direct}, nil
}
