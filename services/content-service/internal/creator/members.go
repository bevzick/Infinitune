package creator

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

type MembershipStore interface {
	Member(context.Context, uuid.UUID, uuid.UUID) (Member, error)
	ListMembers(context.Context, uuid.UUID) ([]Member, error)
	AddMember(context.Context, Member) (Member, error)
	UpdateMemberRole(
		context.Context, uuid.UUID, uuid.UUID, MemberRole, MemberRole,
	) (Member, error)
	RemoveMember(context.Context, uuid.UUID, uuid.UUID, MemberRole) error
}

type UserDirectory interface {
	RequireUser(context.Context, uuid.UUID) error
}

type MemberService struct {
	store MembershipStore
	users UserDirectory
}

func NewMemberService(store MembershipStore, users UserDirectory) *MemberService {
	return &MemberService{store: store, users: users}
}

func (service *MemberService) List(
	ctx context.Context,
	actorID uuid.UUID,
	accountID uuid.UUID,
) ([]Member, error) {
	if _, err := service.authorize(ctx, actorID, accountID, PermissionMembersRead); err != nil {
		return nil, err
	}
	members, err := service.store.ListMembers(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("list creator members: %w", err)
	}
	return members, nil
}

func (service *MemberService) Add(
	ctx context.Context,
	actorID uuid.UUID,
	accountID uuid.UUID,
	userID uuid.UUID,
	role MemberRole,
) (Member, error) {
	actor, err := service.authorize(ctx, actorID, accountID, PermissionMembersManage)
	if err != nil {
		return Member{}, err
	}
	if userID == uuid.Nil {
		return Member{}, validationError("user_id", "must be a UUID")
	}
	if actorID == userID {
		return Member{}, ErrForbidden
	}
	if err = validateManagedRole(actor.Role, role); err != nil {
		return Member{}, err
	}
	if service.users == nil {
		return Member{}, errors.New("creator member user directory is not configured")
	}
	if err = service.users.RequireUser(ctx, userID); err != nil {
		return Member{}, fmt.Errorf("validate creator member user: %w", err)
	}
	added, err := service.store.AddMember(ctx, Member{
		CreatorID: accountID, UserID: userID, Role: role,
	})
	if err != nil {
		return Member{}, fmt.Errorf("add creator member: %w", err)
	}
	return added, nil
}

func (service *MemberService) ChangeRole(
	ctx context.Context,
	actorID uuid.UUID,
	accountID uuid.UUID,
	userID uuid.UUID,
	role MemberRole,
) (Member, error) {
	actor, err := service.authorize(ctx, actorID, accountID, PermissionMembersManage)
	if err != nil {
		return Member{}, err
	}
	if userID == uuid.Nil {
		return Member{}, validationError("user_id", "must be a UUID")
	}
	if actorID == userID {
		return Member{}, ErrForbidden
	}
	target, err := service.store.Member(ctx, accountID, userID)
	if errors.Is(err, ErrNotFound) {
		return Member{}, ErrNotFound
	}
	if err != nil {
		return Member{}, fmt.Errorf("read changed creator member: %w", err)
	}
	if target.Role == RoleOwner {
		return Member{}, ErrOwnerProtected
	}
	if err = validateManagedRole(actor.Role, role); err != nil {
		return Member{}, err
	}
	if !canManageRole(actor.Role, target.Role) {
		return Member{}, ErrForbidden
	}
	if target.Role == role {
		return target, nil
	}
	updated, err := service.store.UpdateMemberRole(ctx, accountID, userID, target.Role, role)
	if err != nil {
		return Member{}, fmt.Errorf("change creator member role: %w", err)
	}
	return updated, nil
}

func (service *MemberService) Remove(
	ctx context.Context,
	actorID uuid.UUID,
	accountID uuid.UUID,
	userID uuid.UUID,
) error {
	actor, err := service.authorize(ctx, actorID, accountID, PermissionMembersManage)
	if err != nil {
		return err
	}
	if userID == uuid.Nil {
		return validationError("user_id", "must be a UUID")
	}
	if actorID == userID {
		return ErrForbidden
	}
	target, err := service.store.Member(ctx, accountID, userID)
	if errors.Is(err, ErrNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read removed creator member: %w", err)
	}
	if target.Role == RoleOwner {
		return ErrOwnerProtected
	}
	if !canManageRole(actor.Role, target.Role) {
		return ErrForbidden
	}
	if err = service.store.RemoveMember(ctx, accountID, userID, target.Role); err != nil {
		return fmt.Errorf("remove creator member: %w", err)
	}
	return nil
}

func (service *MemberService) Authorize(
	ctx context.Context,
	actorID uuid.UUID,
	accountID uuid.UUID,
	permission Permission,
) error {
	_, err := service.authorize(ctx, actorID, accountID, permission)
	return err
}

func (service *MemberService) authorize(
	ctx context.Context,
	actorID uuid.UUID,
	accountID uuid.UUID,
	permission Permission,
) (Member, error) {
	if actorID == uuid.Nil {
		return Member{}, ErrForbidden
	}
	if accountID == uuid.Nil {
		return Member{}, validationError("creator_id", "must be a UUID")
	}
	membership, err := service.store.Member(ctx, accountID, actorID)
	if errors.Is(err, ErrNotFound) {
		return Member{}, ErrForbidden
	}
	if err != nil {
		return Member{}, fmt.Errorf("authorize creator member: %w", err)
	}
	if !HasPermission(membership.Role, permission) {
		return Member{}, ErrForbidden
	}
	return membership, nil
}

func HasPermission(role MemberRole, permission Permission) bool {
	switch role {
	case RoleOwner:
		switch permission {
		case PermissionAccountRead, PermissionAccountUpdate,
			PermissionVerificationSubmit, PermissionMembersRead,
			PermissionMembersManage, PermissionContentEdit,
			PermissionContentUpload, PermissionContentPublish:
			return true
		}
	case RoleManager:
		switch permission {
		case PermissionAccountRead, PermissionMembersRead, PermissionMembersManage,
			PermissionContentEdit, PermissionContentUpload, PermissionContentPublish:
			return true
		}
	case RoleEditor:
		return permission == PermissionAccountRead || permission == PermissionContentEdit
	case RoleUploader:
		return permission == PermissionAccountRead || permission == PermissionContentUpload
	}
	return false
}

func validateManagedRole(actorRole, targetRole MemberRole) error {
	if targetRole == RoleOwner {
		return ErrOwnerProtected
	}
	switch targetRole {
	case RoleManager, RoleEditor, RoleUploader:
		if actorRole == RoleOwner ||
			(actorRole == RoleManager && targetRole != RoleManager) {
			return nil
		}
		return ErrForbidden
	default:
		return validationError("member_role", "must be manager, editor, or uploader")
	}
}

func canManageRole(actorRole, targetRole MemberRole) bool {
	if actorRole == RoleOwner {
		return targetRole != RoleOwner
	}
	return actorRole == RoleManager &&
		(targetRole == RoleEditor || targetRole == RoleUploader)
}
