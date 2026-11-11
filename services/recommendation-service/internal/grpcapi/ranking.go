package grpcapi

import (
	"context"
	"errors"

	rankingv1 "github.com/bevzick/Infinitune/services/recommendation-service/api/ranking/v1"
	"github.com/bevzick/Infinitune/services/recommendation-service/internal/recommendation"
	preferencesignal "github.com/bevzick/Infinitune/services/recommendation-service/internal/signal"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CandidateRanker interface {
	RankCandidates(context.Context, uuid.UUID, []uuid.UUID) ([]recommendation.Item, bool, error)
}

type RankingServer struct {
	rankingv1.UnimplementedRankingServiceServer
	ranker CandidateRanker
}

func NewRankingServer(ranker CandidateRanker) *RankingServer { return &RankingServer{ranker: ranker} }

func (server *RankingServer) RankCandidates(ctx context.Context, request *rankingv1.RankCandidatesRequest) (*rankingv1.RankCandidatesResponse, error) {
	if request == nil || len(request.GetTrackIds()) == 0 || len(request.GetTrackIds()) > 100 {
		return nil, status.Error(codes.InvalidArgument, "1 to 100 candidate track_ids are required")
	}
	userID, err := uuid.Parse(request.GetUserId())
	if err != nil || userID == uuid.Nil {
		return nil, status.Error(codes.InvalidArgument, "user_id must be a UUID")
	}
	trackIDs := make([]uuid.UUID, 0, len(request.GetTrackIds()))
	seen := make(map[uuid.UUID]struct{}, len(request.GetTrackIds()))
	for _, value := range request.GetTrackIds() {
		trackID, parseErr := uuid.Parse(value)
		if parseErr != nil || trackID == uuid.Nil {
			return nil, status.Error(codes.InvalidArgument, "track_ids must contain UUIDs")
		}
		if _, exists := seen[trackID]; exists {
			return nil, status.Error(codes.InvalidArgument, "track_ids must be unique")
		}
		seen[trackID] = struct{}{}
		trackIDs = append(trackIDs, trackID)
	}
	items, personalized, err := server.ranker.RankCandidates(ctx, userID, trackIDs)
	if errors.Is(err, preferencesignal.ErrPersonalizationDisabled) {
		return nil, status.Error(codes.FailedPrecondition, "personalized recommendations are disabled")
	}
	if err != nil {
		return nil, status.Error(codes.Unavailable, "candidate ranking is unavailable")
	}
	tracks := make([]*rankingv1.RankedTrack, 0, len(items))
	for _, item := range items {
		tracks = append(tracks, &rankingv1.RankedTrack{TrackId: item.TrackID.String(), Score: item.Score, Rank: int32(item.Rank), Reason: item.Reason})
	}
	return &rankingv1.RankCandidatesResponse{Tracks: tracks, AlgorithmVersion: "transparent-v1", Personalized: personalized}, nil
}
