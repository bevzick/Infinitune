package creator

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestAccessServiceChecksRoleAndVerification(t *testing.T) {
	creatorID, userID := uuid.New(), uuid.New()
	store := &accessStoreStub{
		account: Account{ID: creatorID, VerificationStatus: VerificationUnverified},
		member:  Member{CreatorID: creatorID, UserID: userID, Role: RoleManager},
	}
	service := NewAccessService(store)

	decision, err := service.Check(context.Background(), userID, creatorID, PermissionContentEdit)
	if err != nil || !decision.Allowed || decision.Role != RoleManager {
		t.Fatalf("Check(edit) = %+v, %v", decision, err)
	}
	decision, err = service.Check(context.Background(), userID, creatorID, PermissionContentPublish)
	if err != nil || decision.Allowed || decision.VerificationStatus != VerificationUnverified {
		t.Fatalf("Check(publish) = %+v, %v", decision, err)
	}
}

func TestAccessServiceDeniesNonMember(t *testing.T) {
	creatorID := uuid.New()
	decision, err := NewAccessService(&accessStoreStub{
		account: Account{ID: creatorID, VerificationStatus: VerificationVerified},
	}).Check(context.Background(), uuid.New(), creatorID, PermissionContentPublish)
	if err != nil || decision.Allowed || decision.Role != "" {
		t.Fatalf("Check() = %+v, %v", decision, err)
	}
}

func TestAccessServiceRejectsUnknownPermission(t *testing.T) {
	_, err := NewAccessService(&accessStoreStub{}).Check(
		context.Background(), uuid.New(), uuid.New(), Permission("creator.unknown"),
	)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("Check() error = %v, want %v", err, ErrInvalid)
	}
}

type accessStoreStub struct {
	account Account
	member  Member
}

func (stub *accessStoreStub) ByID(context.Context, uuid.UUID) (Account, error) {
	return stub.account, nil
}

func (stub *accessStoreStub) Member(_ context.Context, creatorID, userID uuid.UUID) (Member, error) {
	if stub.member.CreatorID != creatorID || stub.member.UserID != userID {
		return Member{}, ErrNotFound
	}
	return stub.member, nil
}
