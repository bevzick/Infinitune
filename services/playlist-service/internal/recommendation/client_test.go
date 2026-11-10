package recommendation

import (
	"context"
	"errors"
	"testing"

	recommendationv1 "github.com/bevzick/Infinitune/services/playlist-service/api/recommendation/v1"
	"github.com/bevzick/Infinitune/services/playlist-service/internal/playlist"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type rpcStub struct {
	response *recommendationv1.RankCandidatesResponse
	err      error
	request  *recommendationv1.RankCandidatesRequest
}

func (stub *rpcStub) RankCandidates(_ context.Context, request *recommendationv1.RankCandidatesRequest, _ ...grpc.CallOption) (*recommendationv1.RankCandidatesResponse, error) {
	stub.request = request
	return stub.response, stub.err
}

func TestClientReturnsValidatedPersonalOrder(t *testing.T) {
	userID, first, second := uuid.New(), uuid.New(), uuid.New()
	rpc := &rpcStub{response: &recommendationv1.RankCandidatesResponse{Tracks: []*recommendationv1.RankedTrack{{TrackId: second.String(), Rank: 1}, {TrackId: first.String(), Rank: 2}}}}
	result, err := NewClient(rpc).Rank(context.Background(), userID, []uuid.UUID{first, second})
	if err != nil {
		t.Fatalf("Rank() error = %v", err)
	}
	if rpc.request.GetUserId() != userID.String() || len(result) != 2 || result[0].TrackID != second || result[1].Position != 2 {
		t.Fatalf("result = %+v; request = %+v", result, rpc.request)
	}
}

func TestClientRejectsForeignTrackAndMapsDisabledPreference(t *testing.T) {
	foreign := &rpcStub{response: &recommendationv1.RankCandidatesResponse{Tracks: []*recommendationv1.RankedTrack{{TrackId: uuid.NewString(), Rank: 1}}}}
	if _, err := NewClient(foreign).Rank(context.Background(), uuid.New(), []uuid.UUID{uuid.New()}); !errors.Is(err, playlist.ErrRecommendationUnavailable) {
		t.Fatalf("foreign response error = %v", err)
	}
	disabled := &rpcStub{err: status.Error(codes.FailedPrecondition, "disabled")}
	if _, err := NewClient(disabled).Rank(context.Background(), uuid.New(), []uuid.UUID{uuid.New()}); !errors.Is(err, playlist.ErrPersonalizationDisabled) {
		t.Fatalf("disabled error = %v", err)
	}
}
