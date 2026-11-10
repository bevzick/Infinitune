package grpcapi

import (
	"context"
	"errors"
	"testing"
	"time"

	userv1 "github.com/bevzick/Infinitune/services/user-service/api/user/v1"
	"github.com/bevzick/Infinitune/services/user-service/internal/profile"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetPublicProfile(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	createdAt := time.Date(2026, time.November, 5, 8, 0, 0, 0, time.UTC)
	server := NewUserServer(publicProfilesStub{result: profile.PublicProfile{
		UserID: userID, Username: "listener", DisplayName: "Listener",
		Bio: "Music lover", CountryCode: "UA", Language: "uk-UA",
		CreatedAt: createdAt, UpdatedAt: createdAt.Add(time.Hour),
	}}, nil)

	response, err := server.GetPublicProfile(context.Background(), &userv1.GetPublicProfileRequest{
		UserId: userID.String(),
	})
	if err != nil {
		t.Fatalf("GetPublicProfile() error = %v", err)
	}
	if response.GetUserId() != userID.String() || response.GetUsername() != "listener" ||
		!response.GetCreatedAt().AsTime().Equal(createdAt) {
		t.Fatalf("GetPublicProfile() = %+v", response)
	}
}

func TestGetPublicProfileMapsErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		request  *userv1.GetPublicProfileRequest
		profiles PublicProfiles
		wantCode codes.Code
	}{
		{
			name: "invalid id", request: &userv1.GetPublicProfileRequest{UserId: "invalid"},
			profiles: publicProfilesStub{}, wantCode: codes.InvalidArgument,
		},
		{
			name: "not found", request: &userv1.GetPublicProfileRequest{UserId: uuid.NewString()},
			profiles: publicProfilesStub{err: profile.ErrNotFound}, wantCode: codes.NotFound,
		},
		{
			name: "repository unavailable", request: &userv1.GetPublicProfileRequest{UserId: uuid.NewString()},
			profiles: publicProfilesStub{err: errors.New("database unavailable")}, wantCode: codes.Internal,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewUserServer(test.profiles, nil).GetPublicProfile(context.Background(), test.request)
			if status.Code(err) != test.wantCode {
				t.Fatalf("GetPublicProfile() code = %s, want %s; error = %v", status.Code(err), test.wantCode, err)
			}
		})
	}
}

func TestGetRecommendationPreferences(t *testing.T) {
	userID, genreID, artistID, moodID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	server := NewUserServer(nil, preferencesStub{result: profile.Preferences{
		RecommendationsEnabled: true, PreferredMoodID: &moodID,
		Genres:  []profile.WeightedPreference{{ID: genreID, Weight: 0.8}},
		Artists: []profile.WeightedPreference{{ID: artistID, Weight: 1}},
	}})
	response, err := server.GetRecommendationPreferences(context.Background(), &userv1.GetRecommendationPreferencesRequest{UserId: userID.String()})
	if err != nil {
		t.Fatalf("GetRecommendationPreferences() error = %v", err)
	}
	if !response.GetRecommendationsEnabled() || response.GetPreferredMoodId() != moodID.String() || response.GetGenres()[0].GetId() != genreID.String() || response.GetArtists()[0].GetId() != artistID.String() {
		t.Fatalf("GetRecommendationPreferences() = %+v", response)
	}
}

type publicProfilesStub struct {
	result profile.PublicProfile
	err    error
}

func (stub publicProfilesStub) Public(context.Context, uuid.UUID) (profile.PublicProfile, error) {
	return stub.result, stub.err
}

type preferencesStub struct {
	result profile.Preferences
	err    error
}

func (stub preferencesStub) PreferencesByUserID(context.Context, uuid.UUID) (profile.Preferences, error) {
	return stub.result, stub.err
}
