package grpcapi

import (
	"context"
	"errors"

	userv1 "github.com/bevzick/Infinitune/services/user-service/api/user/v1"
	"github.com/bevzick/Infinitune/services/user-service/internal/profile"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type PublicProfiles interface {
	Public(context.Context, uuid.UUID) (profile.PublicProfile, error)
}

type RecommendationProfiles interface {
	ByUserID(context.Context, uuid.UUID) (profile.Profile, error)
}

type UserServer struct {
	userv1.UnimplementedUserServiceServer
	profiles        PublicProfiles
	recommendations RecommendationProfiles
}

func NewUserServer(profiles PublicProfiles, recommendations RecommendationProfiles) *UserServer {
	return &UserServer{profiles: profiles, recommendations: recommendations}
}

func (server *UserServer) GetRecommendationPreferences(
	ctx context.Context,
	request *userv1.GetRecommendationPreferencesRequest,
) (*userv1.RecommendationPreferences, error) {
	userID, err := uuid.Parse(request.GetUserId())
	if err != nil || userID == uuid.Nil {
		return nil, status.Error(codes.InvalidArgument, "user_id must be a UUID")
	}
	result, err := server.recommendations.ByUserID(ctx, userID)
	if errors.Is(err, profile.ErrNotFound) {
		return nil, status.Error(codes.NotFound, "preferences were not found")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "preference lookup failed")
	}
	response := &userv1.RecommendationPreferences{
		RecommendationsEnabled: result.Preferences.RecommendationsEnabled,
		ExplicitContentAllowed: result.Settings.ExplicitContent,
		Genres:                 mapPreferences(result.Preferences.Genres),
		Artists:                mapPreferences(result.Preferences.Artists),
	}
	if result.Preferences.PreferredMoodID != nil {
		value := result.Preferences.PreferredMoodID.String()
		response.PreferredMoodId = &value
	}
	return response, nil
}

func mapPreferences(values []profile.WeightedPreference) []*userv1.WeightedPreference {
	result := make([]*userv1.WeightedPreference, len(values))
	for index, value := range values {
		result[index] = &userv1.WeightedPreference{Id: value.ID.String(), Weight: value.Weight}
	}
	return result
}

func (server *UserServer) GetPublicProfile(
	ctx context.Context,
	request *userv1.GetPublicProfileRequest,
) (*userv1.PublicProfile, error) {
	userID, err := uuid.Parse(request.GetUserId())
	if err != nil || userID == uuid.Nil {
		return nil, status.Error(codes.InvalidArgument, "user_id must be a UUID")
	}

	result, err := server.profiles.Public(ctx, userID)
	if errors.Is(err, profile.ErrNotFound) {
		return nil, status.Error(codes.NotFound, "profile was not found")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "profile lookup failed")
	}

	return &userv1.PublicProfile{
		UserId: result.UserID.String(), Username: result.Username,
		DisplayName: result.DisplayName, Bio: result.Bio, AvatarUrl: result.AvatarURL,
		CountryCode: result.CountryCode, Language: result.Language,
		CreatedAt: timestamppb.New(result.CreatedAt),
		UpdatedAt: timestamppb.New(result.UpdatedAt),
	}, nil
}
