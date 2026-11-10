package grpcapi

import (
	"context"
	"testing"

	rankingv1 "github.com/bevzick/Infinitune/services/recommendation-service/api/ranking/v1"
	"github.com/bevzick/Infinitune/services/recommendation-service/internal/recommendation"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type rankerStub struct {
	items        []recommendation.Item
	personalized bool
	userID       uuid.UUID
	trackIDs     []uuid.UUID
}

func (stub *rankerStub) RankCandidates(_ context.Context, userID uuid.UUID, trackIDs []uuid.UUID) ([]recommendation.Item, bool, error) {
	stub.userID, stub.trackIDs = userID, trackIDs
	return stub.items, stub.personalized, nil
}

func TestRankingServerMapsCandidateOrder(t *testing.T) {
	userID, first, second := uuid.New(), uuid.New(), uuid.New()
	stub := &rankerStub{personalized: true, items: []recommendation.Item{{TrackID: second, Rank: 1, Score: 4, Reason: "preferred artist"}, {TrackID: first, Rank: 2, Score: 1, Reason: "catalog discovery"}}}
	response, err := NewRankingServer(stub).RankCandidates(context.Background(), &rankingv1.RankCandidatesRequest{UserId: userID.String(), TrackIds: []string{first.String(), second.String()}})
	if err != nil {
		t.Fatalf("RankCandidates() error = %v", err)
	}
	if stub.userID != userID || len(stub.trackIDs) != 2 || len(response.GetTracks()) != 2 || response.GetTracks()[0].GetTrackId() != second.String() || !response.GetPersonalized() {
		t.Fatalf("response = %+v; stub = %+v", response, stub)
	}
}

func TestRankingServerRejectsDuplicateCandidates(t *testing.T) {
	id := uuid.NewString()
	_, err := NewRankingServer(&rankerStub{}).RankCandidates(context.Background(), &rankingv1.RankCandidatesRequest{UserId: uuid.NewString(), TrackIds: []string{id, id}})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %s; error = %v", status.Code(err), err)
	}
}
