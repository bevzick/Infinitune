package signal

import (
	"context"
	"fmt"

	historyv1 "github.com/bevzick/Infinitune/services/recommendation-service/api/history/v1"
	interactionv1 "github.com/bevzick/Infinitune/services/recommendation-service/api/interaction/v1"
	userv1 "github.com/bevzick/Infinitune/services/recommendation-service/api/user/v1"
	"github.com/google/uuid"
)

type UserClient struct{ client userv1.UserServiceClient }

func NewUserClient(client userv1.UserServiceClient) *UserClient { return &UserClient{client: client} }
func (client *UserClient) Preferences(ctx context.Context, userID uuid.UUID) (Preferences, error) {
	response, err := client.client.GetRecommendationPreferences(ctx, &userv1.GetRecommendationPreferencesRequest{UserId: userID.String()})
	if err != nil {
		return Preferences{}, err
	}
	result := Preferences{
		Enabled:         response.GetRecommendationsEnabled(),
		ExplicitAllowed: response.GetExplicitContentAllowed(),
	}
	if response.PreferredMoodId != nil {
		id, parseErr := uuid.Parse(response.GetPreferredMoodId())
		if parseErr != nil || id == uuid.Nil {
			return Preferences{}, fmt.Errorf("user service returned invalid preferred mood id")
		}
		result.PreferredMoodID = &id
	}
	result.Genres, err = parseWeighted(response.GetGenres())
	if err != nil {
		return Preferences{}, fmt.Errorf("parse genre preferences: %w", err)
	}
	result.Artists, err = parseWeighted(response.GetArtists())
	if err != nil {
		return Preferences{}, fmt.Errorf("parse artist preferences: %w", err)
	}
	return result, nil
}
func parseWeighted(values []*userv1.WeightedPreference) ([]WeightedPreference, error) {
	result := make([]WeightedPreference, len(values))
	for index, value := range values {
		id, err := uuid.Parse(value.GetId())
		if err != nil || id == uuid.Nil {
			return nil, fmt.Errorf("invalid preference id")
		}
		result[index] = WeightedPreference{ID: id, Weight: value.GetWeight()}
	}
	return result, nil
}

type InteractionClient struct {
	client interactionv1.SignalServiceClient
}

func NewInteractionClient(client interactionv1.SignalServiceClient) *InteractionClient {
	return &InteractionClient{client: client}
}
func (client *InteractionClient) Interactions(ctx context.Context, userID uuid.UUID, limit int) (Interactions, error) {
	response, err := client.client.ListPreferenceSignals(ctx, &interactionv1.ListPreferenceSignalsRequest{UserId: userID.String(), LimitPerType: int32(limit)})
	if err != nil {
		return Interactions{}, err
	}
	tracks, err := parseIDs(response.GetLikedTrackIds())
	if err != nil {
		return Interactions{}, fmt.Errorf("parse liked track ids: %w", err)
	}
	albums, err := parseIDs(response.GetLikedAlbumIds())
	if err != nil {
		return Interactions{}, fmt.Errorf("parse liked album ids: %w", err)
	}
	artists, err := parseIDs(response.GetFollowedArtistIds())
	if err != nil {
		return Interactions{}, fmt.Errorf("parse followed artist ids: %w", err)
	}
	return Interactions{LikedTrackIDs: tracks, LikedAlbumIDs: albums, FollowedArtistIDs: artists}, nil
}
func parseIDs(values []string) ([]uuid.UUID, error) {
	result := make([]uuid.UUID, len(values))
	for index, value := range values {
		id, err := uuid.Parse(value)
		if err != nil || id == uuid.Nil {
			return nil, fmt.Errorf("invalid UUID %q", value)
		}
		result[index] = id
	}
	return result, nil
}

type HistoryClient struct{ client historyv1.SignalServiceClient }

func NewHistoryClient(client historyv1.SignalServiceClient) *HistoryClient {
	return &HistoryClient{client: client}
}
func (client *HistoryClient) Listening(ctx context.Context, userID uuid.UUID, limit int) ([]Listen, error) {
	response, err := client.client.ListListeningSignals(ctx, &historyv1.ListListeningSignalsRequest{UserId: userID.String(), Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	result := make([]Listen, len(response.GetItems()))
	for index, item := range response.GetItems() {
		id, parseErr := uuid.Parse(item.GetTrackId())
		if parseErr != nil || id == uuid.Nil {
			return nil, fmt.Errorf("history service returned invalid track id")
		}
		result[index] = Listen{TrackID: id, ListenedMS: int(item.GetListenedMs()), Completed: item.GetCompleted()}
	}
	return result, nil
}
