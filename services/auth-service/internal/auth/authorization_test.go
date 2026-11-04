package auth

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestHasPermission(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		roles      []string
		permission Permission
		want       bool
	}{
		{name: "admin reads roles", roles: []string{RoleAdmin}, permission: PermissionReadGlobalRoles, want: true},
		{name: "admin manages roles", roles: []string{RoleListener, RoleAdmin}, permission: PermissionManageGlobalRoles, want: true},
		{name: "moderator cannot manage roles", roles: []string{RoleModerator}, permission: PermissionManageGlobalRoles},
		{name: "creator cannot manage roles", roles: []string{RoleCreator}, permission: PermissionManageGlobalRoles},
		{name: "listener cannot read roles", roles: []string{RoleListener}, permission: PermissionReadGlobalRoles},
		{name: "unknown permission denied", roles: []string{RoleAdmin}, permission: Permission("unknown")},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := HasPermission(test.roles, test.permission); got != test.want {
				t.Fatalf("HasPermission(%v, %q) = %v, want %v", test.roles, test.permission, got, test.want)
			}
		})
	}
}

func TestRoleServiceRequiresAdminInTokenAndDatabase(t *testing.T) {
	t.Parallel()

	actorID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	targetID := uuid.MustParse("22222222-2222-4222-8222-222222222222")

	tests := []struct {
		name       string
		tokenRoles []string
		dbRoles    []string
		status     AccountStatus
	}{
		{name: "listener token", tokenRoles: []string{RoleListener}, dbRoles: []string{RoleAdmin}, status: AccountStatusActive},
		{name: "stale admin token", tokenRoles: []string{RoleAdmin}, dbRoles: []string{RoleListener}, status: AccountStatusActive},
		{name: "blocked admin", tokenRoles: []string{RoleAdmin}, dbRoles: []string{RoleAdmin}, status: AccountStatusBlocked},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			store := newRoleStore()
			store.users[actorID] = User{ID: actorID, Status: test.status}
			store.users[targetID] = User{ID: targetID, Status: AccountStatusActive}
			store.roles[actorID] = test.dbRoles
			store.roles[targetID] = []string{RoleListener}
			service := NewRoleService(store)

			_, err := service.ListUserRoles(
				context.Background(),
				Identity{UserID: actorID, Roles: test.tokenRoles},
				targetID,
			)
			if !errors.Is(err, ErrForbidden) {
				t.Fatalf("ListUserRoles() error = %v, want ErrForbidden", err)
			}
		})
	}
}

func TestRoleServiceAssignsAndRemovesRolesIdempotently(t *testing.T) {
	t.Parallel()

	actorID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	targetID := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	store := newRoleStore()
	store.users[actorID] = User{ID: actorID, Status: AccountStatusActive}
	store.users[targetID] = User{ID: targetID, Status: AccountStatusActive}
	store.roles[actorID] = []string{RoleListener, RoleAdmin}
	store.roles[targetID] = []string{RoleListener}
	service := NewRoleService(store)
	actor := Identity{UserID: actorID, Roles: []string{RoleListener, RoleAdmin}}

	roles, err := service.AssignRole(context.Background(), actor, targetID, RoleCreator)
	if err != nil {
		t.Fatalf("AssignRole() error = %v", err)
	}
	if !slices.Equal(roles, []string{RoleListener, RoleCreator}) {
		t.Fatalf("AssignRole() roles = %v", roles)
	}
	roles, err = service.AssignRole(context.Background(), actor, targetID, RoleCreator)
	if err != nil || !slices.Equal(roles, []string{RoleListener, RoleCreator}) {
		t.Fatalf("AssignRole(second) = %v, %v", roles, err)
	}

	roles, err = service.RemoveRole(context.Background(), actor, targetID, RoleCreator)
	if err != nil || !slices.Equal(roles, []string{RoleListener}) {
		t.Fatalf("RemoveRole() = %v, %v", roles, err)
	}
	roles, err = service.RemoveRole(context.Background(), actor, targetID, RoleCreator)
	if err != nil || !slices.Equal(roles, []string{RoleListener}) {
		t.Fatalf("RemoveRole(second) = %v, %v", roles, err)
	}
}

func TestRoleServiceProtectsRoleBoundaries(t *testing.T) {
	t.Parallel()

	actorID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	targetID := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	store := newRoleStore()
	store.users[actorID] = User{ID: actorID, Status: AccountStatusActive}
	store.users[targetID] = User{ID: targetID, Status: AccountStatusActive}
	store.roles[actorID] = []string{RoleListener, RoleAdmin}
	store.roles[targetID] = []string{RoleListener}
	service := NewRoleService(store)
	actor := Identity{UserID: actorID, Roles: []string{RoleAdmin}}

	if _, err := service.AssignRole(context.Background(), actor, targetID, "owner"); !errors.Is(err, ErrInvalidRole) {
		t.Fatalf("AssignRole(invalid) error = %v", err)
	}
	if _, err := service.RemoveRole(context.Background(), actor, targetID, RoleListener); !errors.Is(err, ErrProtectedRole) {
		t.Fatalf("RemoveRole(listener) error = %v", err)
	}
	if _, err := service.RemoveRole(context.Background(), actor, actorID, RoleAdmin); !errors.Is(err, ErrCannotRemoveOwnAdminRole) {
		t.Fatalf("RemoveRole(own admin) error = %v", err)
	}

	store.removeError = ErrCannotRemoveLastAdmin
	if _, err := service.RemoveRole(context.Background(), actor, targetID, RoleAdmin); !errors.Is(err, ErrCannotRemoveLastAdmin) {
		t.Fatalf("RemoveRole(last admin) error = %v", err)
	}
}

type roleStore struct {
	users       map[uuid.UUID]User
	roles       map[uuid.UUID][]string
	removeError error
}

func newRoleStore() *roleStore {
	return &roleStore{
		users: make(map[uuid.UUID]User),
		roles: make(map[uuid.UUID][]string),
	}
}

func (s *roleStore) FindUserByID(_ context.Context, userID uuid.UUID) (User, error) {
	user, exists := s.users[userID]
	if !exists {
		return User{}, ErrUserNotFound
	}
	return user, nil
}

func (s *roleStore) ListRoleNamesForUser(_ context.Context, userID uuid.UUID) ([]string, error) {
	return append([]string(nil), s.roles[userID]...), nil
}

func (s *roleStore) AssignRoleToUser(
	_ context.Context,
	userID uuid.UUID,
	role string,
	_ time.Time,
) error {
	if slices.Contains(s.roles[userID], role) {
		return ErrRoleAlreadyAssigned
	}
	s.roles[userID] = append(s.roles[userID], role)
	return nil
}

func (s *roleStore) RemoveRoleFromUser(_ context.Context, userID uuid.UUID, role string) error {
	if s.removeError != nil {
		return s.removeError
	}
	index := slices.Index(s.roles[userID], role)
	if index < 0 {
		return ErrRoleNotAssigned
	}
	s.roles[userID] = slices.Delete(s.roles[userID], index, index+1)
	return nil
}
