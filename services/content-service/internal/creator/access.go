package creator

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

type AccessStore interface {
	ByID(context.Context, uuid.UUID) (Account, error)
	Member(context.Context, uuid.UUID, uuid.UUID) (Member, error)
}

type AccessService struct {
	store AccessStore
}

func NewAccessService(store AccessStore) *AccessService {
	return &AccessService{store: store}
}

func (service *AccessService) Check(
	ctx context.Context,
	userID uuid.UUID,
	accountID uuid.UUID,
	permission Permission,
) (AccessDecision, error) {
	if userID == uuid.Nil {
		return AccessDecision{}, validationError("user_id", "must be a UUID")
	}
	if accountID == uuid.Nil {
		return AccessDecision{}, validationError("creator_id", "must be a UUID")
	}
	if !validPermission(permission) {
		return AccessDecision{}, validationError("permission", "is not supported")
	}
	account, err := service.store.ByID(ctx, accountID)
	if err != nil {
		return AccessDecision{}, fmt.Errorf("read permission creator account: %w", err)
	}
	decision := AccessDecision{VerificationStatus: account.VerificationStatus}
	membership, err := service.store.Member(ctx, accountID, userID)
	if errors.Is(err, ErrNotFound) {
		return decision, nil
	}
	if err != nil {
		return AccessDecision{}, fmt.Errorf("read permission membership: %w", err)
	}
	decision.Role = membership.Role
	decision.Allowed = HasPermission(membership.Role, permission)
	if permission == PermissionContentPublish &&
		account.VerificationStatus != VerificationVerified {
		decision.Allowed = false
	}
	return decision, nil
}

func validPermission(permission Permission) bool {
	switch permission {
	case PermissionAccountRead, PermissionAccountUpdate,
		PermissionVerificationSubmit, PermissionMembersRead,
		PermissionMembersManage, PermissionContentEdit,
		PermissionContentUpload, PermissionContentPublish:
		return true
	default:
		return false
	}
}
