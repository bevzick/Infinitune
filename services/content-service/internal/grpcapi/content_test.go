package grpcapi

import (
	"context"
	"testing"

	contentv1 "github.com/bevzick/Infinitune/services/content-service/api/content/v1"
	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestContentServerReturnsAccessDecision(t *testing.T) {
	checker := &checkerStub{decision: creator.AccessDecision{
		Allowed: true, Role: creator.RoleOwner,
		VerificationStatus: creator.VerificationVerified,
	}}
	response, err := NewContentServer(checker).CheckPermission(context.Background(),
		&contentv1.CheckPermissionRequest{
			CreatorId: uuid.NewString(), UserId: uuid.NewString(),
			Permission: string(creator.PermissionContentPublish),
		})
	if err != nil || !response.GetAllowed() || response.GetMemberRole() != "owner" {
		t.Fatalf("CheckPermission() = %+v, %v", response, err)
	}
}

func TestContentServerRejectsInvalidIdentifiers(t *testing.T) {
	_, err := NewContentServer(&checkerStub{}).CheckPermission(context.Background(),
		&contentv1.CheckPermissionRequest{CreatorId: "bad", UserId: uuid.NewString()})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("CheckPermission() code = %v", status.Code(err))
	}
}

type checkerStub struct {
	decision creator.AccessDecision
	err      error
}

func (stub *checkerStub) Check(
	context.Context, uuid.UUID, uuid.UUID, creator.Permission,
) (creator.AccessDecision, error) {
	return stub.decision, stub.err
}
