package signal

import (
	"context"
	"errors"
	"testing"

	userv1 "github.com/bevzick/Infinitune/services/recommendation-service/api/user/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestUserClientUsesColdStartPreferencesWhenProfileIsMissing(t *testing.T) {
	client := NewUserClient(userClientStub{err: status.Error(codes.NotFound, "preferences were not found")})

	preferences, err := client.Preferences(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("Preferences() error = %v", err)
	}
	if !preferences.Enabled {
		t.Fatal("Preferences().Enabled = false, want cold-start recommendations enabled")
	}
	if preferences.ExplicitAllowed {
		t.Fatal("Preferences().ExplicitAllowed = true, want privacy-safe default")
	}
	if preferences.PreferredMoodID != nil || len(preferences.Genres) != 0 || len(preferences.Artists) != 0 {
		t.Fatalf("Preferences() = %+v, want empty cold-start signals", preferences)
	}
}

func TestUserClientPreservesUpstreamFailures(t *testing.T) {
	upstreamErr := status.Error(codes.Unavailable, "user service unavailable")
	client := NewUserClient(userClientStub{err: upstreamErr})

	_, err := client.Preferences(context.Background(), uuid.New())
	if !errors.Is(err, upstreamErr) {
		t.Fatalf("Preferences() error = %v, want %v", err, upstreamErr)
	}
}

type userClientStub struct {
	response *userv1.RecommendationPreferences
	err      error
}

func (stub userClientStub) GetRecommendationPreferences(
	context.Context,
	*userv1.GetRecommendationPreferencesRequest,
	...grpc.CallOption,
) (*userv1.RecommendationPreferences, error) {
	return stub.response, stub.err
}
