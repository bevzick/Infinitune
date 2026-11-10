package recommendation

import (
	"context"
	"errors"
	"fmt"
	"time"

	recommendationv1 "github.com/bevzick/Infinitune/services/playlist-service/api/recommendation/v1"
	"github.com/bevzick/Infinitune/services/playlist-service/internal/playlist"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const rankingTimeout = 3 * time.Second

type rankingRPC interface {
	RankCandidates(context.Context, *recommendationv1.RankCandidatesRequest, ...grpc.CallOption) (*recommendationv1.RankCandidatesResponse, error)
}

type Client struct {
	rpc     rankingRPC
	timeout time.Duration
}

func NewClient(rpc rankingRPC) *Client { return &Client{rpc: rpc, timeout: rankingTimeout} }

func (client *Client) Rank(ctx context.Context, userID uuid.UUID, trackIDs []uuid.UUID) ([]playlist.DynamicTrack, error) {
	ids := make([]string, len(trackIDs))
	for index, trackID := range trackIDs {
		ids[index] = trackID.String()
	}
	callCtx, cancel := context.WithTimeout(ctx, client.timeout)
	defer cancel()
	response, err := client.rpc.RankCandidates(callCtx, &recommendationv1.RankCandidatesRequest{UserId: userID.String(), TrackIds: ids})
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if errors.Is(callCtx.Err(), context.DeadlineExceeded) || status.Code(err) == codes.DeadlineExceeded {
			return nil, playlist.ErrRecommendationTimeout
		}
		if status.Code(err) == codes.FailedPrecondition {
			return nil, playlist.ErrPersonalizationDisabled
		}
		return nil, fmt.Errorf("%w: RankCandidates: %v", playlist.ErrRecommendationUnavailable, err)
	}
	allowed := make(map[uuid.UUID]struct{}, len(trackIDs))
	for _, id := range trackIDs {
		allowed[id] = struct{}{}
	}
	result := make([]playlist.DynamicTrack, 0, len(response.GetTracks()))
	seen := make(map[uuid.UUID]struct{}, len(response.GetTracks()))
	for index, track := range response.GetTracks() {
		id, parseErr := uuid.Parse(track.GetTrackId())
		if parseErr != nil || id == uuid.Nil || track.GetRank() != int32(index+1) {
			return nil, fmt.Errorf("%w: malformed ranking response", playlist.ErrRecommendationUnavailable)
		}
		if _, exists := allowed[id]; !exists {
			return nil, fmt.Errorf("%w: ranking introduced a foreign track", playlist.ErrRecommendationUnavailable)
		}
		if _, exists := seen[id]; exists {
			return nil, fmt.Errorf("%w: ranking duplicated a track", playlist.ErrRecommendationUnavailable)
		}
		seen[id] = struct{}{}
		result = append(result, playlist.DynamicTrack{TrackID: id, Position: index + 1})
	}
	return result, nil
}
