package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Permission string

const (
	PermissionReadGlobalRoles   Permission = "auth.roles.read"
	PermissionManageGlobalRoles Permission = "auth.roles.manage"
)

var (
	ErrForbidden                = errors.New("operation is forbidden")
	ErrInvalidRole              = errors.New("role is invalid")
	ErrProtectedRole            = errors.New("role is managed by the account lifecycle")
	ErrUserNotFound             = errors.New("user not found")
	ErrRoleUnavailable          = errors.New("role is unavailable")
	ErrRoleAlreadyAssigned      = errors.New("role is already assigned")
	ErrRoleNotAssigned          = errors.New("role is not assigned")
	ErrCannotRemoveOwnAdminRole = errors.New("cannot remove own admin role")
	ErrCannotRemoveLastAdmin    = errors.New("cannot remove last admin role")
	ErrInvalidAccountStatus     = errors.New("account status is invalid")
	ErrInvalidUserQuery         = errors.New("user query is invalid")
	ErrCannotChangeOwnStatus    = errors.New("admin cannot change their own account status")
	ErrCannotDisableLastAdmin   = errors.New("cannot disable the last active admin")
)

type RoleAssignmentStore interface {
	FindUserByID(context.Context, uuid.UUID) (User, error)
	ListRoleNamesForUser(context.Context, uuid.UUID) ([]string, error)
	AssignRoleToUser(context.Context, uuid.UUID, string, time.Time) error
	RemoveRoleFromUser(context.Context, uuid.UUID, string) error
	ListUsers(context.Context, UserPageRequest) (UserPage, error)
	UpdateUserStatus(context.Context, uuid.UUID, AccountStatus, time.Time) (User, error)
}

func (s *RoleService) ListUsers(
	ctx context.Context,
	actor Identity,
	request UserPageRequest,
) (UserPage, error) {
	if err := s.authorize(ctx, actor, PermissionReadGlobalRoles); err != nil {
		return UserPage{}, err
	}
	if request.Limit == 0 {
		request.Limit = 20
	}
	if request.Limit < 1 || request.Limit > 100 || request.Offset < 0 {
		return UserPage{}, ErrInvalidUserQuery
	}
	if request.Status != nil && !validAccountStatus(*request.Status, true) {
		return UserPage{}, ErrInvalidAccountStatus
	}
	page, err := s.store.ListUsers(ctx, request)
	if err != nil {
		return UserPage{}, fmt.Errorf("list users: %w", err)
	}
	return page, nil
}

func (s *RoleService) UpdateUserStatus(
	ctx context.Context,
	actor Identity,
	userID uuid.UUID,
	status AccountStatus,
) (User, error) {
	if err := s.authorize(ctx, actor, PermissionManageGlobalRoles); err != nil {
		return User{}, err
	}
	if userID == uuid.Nil || !validAccountStatus(status, false) {
		return User{}, ErrInvalidAccountStatus
	}
	if actor.UserID == userID {
		return User{}, ErrCannotChangeOwnStatus
	}
	user, err := s.store.UpdateUserStatus(ctx, userID, status, time.Now().UTC())
	if err != nil {
		return User{}, fmt.Errorf("update user status: %w", err)
	}
	return user, nil
}

func validAccountStatus(status AccountStatus, allowDeleted bool) bool {
	return status == AccountStatusActive || status == AccountStatusBlocked ||
		(allowDeleted && status == AccountStatusDeleted)
}

type RoleService struct {
	store RoleAssignmentStore
}

func NewRoleService(store RoleAssignmentStore) *RoleService {
	return &RoleService{store: store}
}

func HasPermission(roles []string, permission Permission) bool {
	switch permission {
	case PermissionReadGlobalRoles, PermissionManageGlobalRoles:
		for _, role := range roles {
			if role == RoleAdmin {
				return true
			}
		}
	}

	return false
}

func (s *RoleService) ListUserRoles(
	ctx context.Context,
	actor Identity,
	userID uuid.UUID,
) ([]string, error) {
	if err := s.authorize(ctx, actor, PermissionReadGlobalRoles); err != nil {
		return nil, err
	}

	return s.loadUserRoles(ctx, userID)
}

func (s *RoleService) AssignRole(
	ctx context.Context,
	actor Identity,
	userID uuid.UUID,
	role string,
) ([]string, error) {
	if err := s.authorize(ctx, actor, PermissionManageGlobalRoles); err != nil {
		return nil, err
	}
	if err := validateAssignableRole(role); err != nil {
		return nil, err
	}
	if _, err := s.loadUserRoles(ctx, userID); err != nil {
		return nil, err
	}

	err := s.store.AssignRoleToUser(ctx, userID, role, time.Now().UTC())
	if err != nil && !errors.Is(err, ErrRoleAlreadyAssigned) {
		return nil, fmt.Errorf("assign user role: %w", err)
	}

	return s.loadUserRoles(ctx, userID)
}

func (s *RoleService) RemoveRole(
	ctx context.Context,
	actor Identity,
	userID uuid.UUID,
	role string,
) ([]string, error) {
	if err := s.authorize(ctx, actor, PermissionManageGlobalRoles); err != nil {
		return nil, err
	}
	if err := validateAssignableRole(role); err != nil {
		return nil, err
	}
	if actor.UserID == userID && role == RoleAdmin {
		return nil, ErrCannotRemoveOwnAdminRole
	}
	if _, err := s.loadUserRoles(ctx, userID); err != nil {
		return nil, err
	}

	err := s.store.RemoveRoleFromUser(ctx, userID, role)
	if err != nil && !errors.Is(err, ErrRoleNotAssigned) {
		return nil, fmt.Errorf("remove user role: %w", err)
	}

	return s.loadUserRoles(ctx, userID)
}

func (s *RoleService) authorize(
	ctx context.Context,
	actor Identity,
	permission Permission,
) error {
	if !HasPermission(actor.Roles, permission) {
		return ErrForbidden
	}

	user, err := s.store.FindUserByID(ctx, actor.UserID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return ErrForbidden
		}
		return fmt.Errorf("load authorization user: %w", err)
	}
	if user.Status != AccountStatusActive {
		return ErrForbidden
	}

	roles, err := s.store.ListRoleNamesForUser(ctx, actor.UserID)
	if err != nil {
		return fmt.Errorf("load authorization roles: %w", err)
	}
	if !HasPermission(roles, permission) {
		return ErrForbidden
	}

	return nil
}

func (s *RoleService) loadUserRoles(ctx context.Context, userID uuid.UUID) ([]string, error) {
	if _, err := s.store.FindUserByID(ctx, userID); err != nil {
		return nil, err
	}

	roles, err := s.store.ListRoleNamesForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list user roles: %w", err)
	}

	return roles, nil
}

func validateAssignableRole(role string) error {
	switch role {
	case RoleAdmin, RoleModerator, RoleCreator:
		return nil
	case RoleListener:
		return ErrProtectedRole
	default:
		return ErrInvalidRole
	}
}
