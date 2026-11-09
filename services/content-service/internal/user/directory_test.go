package user

import (
	"context"
	"errors"
	"testing"

	userv1 "github.com/bevzick/Infinitune/services/content-service/api/user/v1"
	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestDirectoryRequiresExistingUser(t *testing.T) {
	userID := uuid.New()
	err := NewDirectory(profileClientStub{profile: &userv1.PublicProfile{
		UserId: userID.String(),
	}}).RequireUser(context.Background(), userID)
	if err != nil {
		t.Fatalf("RequireUser() error = %v", err)
	}
}

func TestDirectoryMapsMissingUser(t *testing.T) {
	err := NewDirectory(profileClientStub{err: status.Error(codes.NotFound, "missing")}).
		RequireUser(context.Background(), uuid.New())
	if !errors.Is(err, creator.ErrTargetUserNotFound) {
		t.Fatalf("RequireUser() error = %v", err)
	}
}

func TestDirectoryPreservesCallerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := NewDirectory(profileClientStub{err: status.Error(codes.Canceled, "canceled")}).
		RequireUser(ctx, uuid.New())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RequireUser() error = %v", err)
	}
}

type profileClientStub struct {
	profile *userv1.PublicProfile
	err     error
}

func (stub profileClientStub) GetPublicProfile(
	context.Context, *userv1.GetPublicProfileRequest, ...grpc.CallOption,
) (*userv1.PublicProfile, error) {
	return stub.profile, stub.err
}
