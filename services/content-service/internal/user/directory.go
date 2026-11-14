package user

import (
	"context"
	"fmt"
	"time"

	userv1 "github.com/bevzick/Infinitune/services/content-service/api/user/v1"
	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const requestTimeout = 3 * time.Second

type ProfileClient interface {
	GetPublicProfile(context.Context, *userv1.GetPublicProfileRequest, ...grpc.CallOption) (*userv1.PublicProfile, error)
}

type Directory struct{ client ProfileClient }

func NewDirectory(client ProfileClient) *Directory { return &Directory{client: client} }

func (directory *Directory) RequireUser(ctx context.Context, userID uuid.UUID) error {
	requestCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	profile, err := directory.client.GetPublicProfile(requestCtx,
		&userv1.GetPublicProfileRequest{UserId: userID.String()})
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if status.Code(err) == codes.NotFound {
			return creator.ErrTargetUserNotFound
		}
		return fmt.Errorf("%w: %v", creator.ErrUserDirectoryUnavailable, err)
	}
	returnedID, parseErr := uuid.Parse(profile.GetUserId())
	if parseErr != nil || returnedID != userID {
		return fmt.Errorf("%w: user service returned an invalid profile", creator.ErrUserDirectoryUnavailable)
	}
	return nil
}

var _ creator.UserDirectory = (*Directory)(nil)
