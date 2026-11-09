package creator

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"
)

func TestMembershipPermissionMatrix(t *testing.T) {
	tests := []struct {
		role    MemberRole
		allowed []Permission
	}{
		{role: RoleOwner, allowed: []Permission{
			PermissionAccountRead, PermissionAccountUpdate, PermissionVerificationSubmit,
			PermissionMembersRead, PermissionMembersManage, PermissionContentEdit,
			PermissionContentUpload, PermissionContentPublish,
		}},
		{role: RoleManager, allowed: []Permission{
			PermissionAccountRead, PermissionMembersRead, PermissionMembersManage,
			PermissionContentEdit, PermissionContentUpload, PermissionContentPublish,
		}},
		{role: RoleEditor, allowed: []Permission{
			PermissionAccountRead, PermissionContentEdit,
		}},
		{role: RoleUploader, allowed: []Permission{
			PermissionAccountRead, PermissionContentUpload,
		}},
	}
	all := []Permission{
		PermissionAccountRead, PermissionAccountUpdate, PermissionVerificationSubmit,
		PermissionMembersRead, PermissionMembersManage, PermissionContentEdit,
		PermissionContentUpload, PermissionContentPublish, Permission("unknown"),
	}
	for _, test := range tests {
		t.Run(string(test.role), func(t *testing.T) {
			for _, permission := range all {
				if got := HasPermission(test.role, permission); got != slices.Contains(test.allowed, permission) {
					t.Fatalf("HasPermission(%q, %q) = %t", test.role, permission, got)
				}
			}
		})
	}
	if HasPermission(MemberRole("listener"), PermissionAccountRead) {
		t.Fatal("unknown membership role received a permission")
	}
}

func TestMemberServiceAddsMembersWithinActorScope(t *testing.T) {
	accountID := uuid.New()
	ownerID := uuid.New()
	managerID := uuid.New()
	tests := []struct {
		name      string
		actor     Member
		targetID  uuid.UUID
		role      MemberRole
		wantError error
	}{
		{name: "owner adds manager", actor: Member{
			CreatorID: accountID, UserID: ownerID, Role: RoleOwner,
		}, targetID: uuid.New(), role: RoleManager},
		{name: "manager adds editor", actor: Member{
			CreatorID: accountID, UserID: managerID, Role: RoleManager,
		}, targetID: uuid.New(), role: RoleEditor},
		{name: "manager adds uploader", actor: Member{
			CreatorID: accountID, UserID: managerID, Role: RoleManager,
		}, targetID: uuid.New(), role: RoleUploader},
		{name: "manager cannot add manager", actor: Member{
			CreatorID: accountID, UserID: managerID, Role: RoleManager,
		}, targetID: uuid.New(), role: RoleManager, wantError: ErrForbidden},
		{name: "owner role is protected", actor: Member{
			CreatorID: accountID, UserID: ownerID, Role: RoleOwner,
		}, targetID: uuid.New(), role: RoleOwner, wantError: ErrOwnerProtected},
		{name: "unknown role", actor: Member{
			CreatorID: accountID, UserID: ownerID, Role: RoleOwner,
		}, targetID: uuid.New(), role: "publisher", wantError: ErrInvalid},
		{name: "self assignment", actor: Member{
			CreatorID: accountID, UserID: ownerID, Role: RoleOwner,
		}, targetID: ownerID, role: RoleManager, wantError: ErrForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := newMembershipStore(test.actor)
			users := &userDirectoryStub{}
			added, err := NewMemberService(store, users).Add(
				context.Background(), test.actor.UserID, accountID, test.targetID, test.role,
			)
			if !errors.Is(err, test.wantError) {
				t.Fatalf("Add() error = %v, want %v", err, test.wantError)
			}
			if test.wantError != nil {
				if store.addCalls != 0 {
					t.Fatalf("AddMember() calls = %d", store.addCalls)
				}
				return
			}
			if added.CreatorID != accountID || added.UserID != test.targetID ||
				added.Role != test.role || users.userID != test.targetID {
				t.Fatalf("Add() = %+v, validated user = %s", added, users.userID)
			}
		})
	}
}

func TestMemberServiceRequiresExistingTargetUser(t *testing.T) {
	accountID := uuid.New()
	ownerID := uuid.New()
	store := newMembershipStore(Member{
		CreatorID: accountID, UserID: ownerID, Role: RoleOwner,
	})
	service := NewMemberService(store, &userDirectoryStub{err: ErrTargetUserNotFound})

	_, err := service.Add(context.Background(), ownerID, accountID, uuid.New(), RoleEditor)
	if !errors.Is(err, ErrTargetUserNotFound) {
		t.Fatalf("Add() error = %v, want %v", err, ErrTargetUserNotFound)
	}
	if store.addCalls != 0 {
		t.Fatalf("AddMember() calls = %d", store.addCalls)
	}
}

func TestMemberServiceChangesRolesWithoutEscalation(t *testing.T) {
	accountID := uuid.New()
	ownerID := uuid.New()
	managerID := uuid.New()
	targetID := uuid.New()
	tests := []struct {
		name       string
		actor      Member
		targetRole MemberRole
		newRole    MemberRole
		wantError  error
	}{
		{name: "owner changes manager to editor", actor: Member{
			CreatorID: accountID, UserID: ownerID, Role: RoleOwner,
		}, targetRole: RoleManager, newRole: RoleEditor},
		{name: "manager changes editor to uploader", actor: Member{
			CreatorID: accountID, UserID: managerID, Role: RoleManager,
		}, targetRole: RoleEditor, newRole: RoleUploader},
		{name: "manager cannot manage manager", actor: Member{
			CreatorID: accountID, UserID: managerID, Role: RoleManager,
		}, targetRole: RoleManager, newRole: RoleEditor, wantError: ErrForbidden},
		{name: "manager cannot promote to manager", actor: Member{
			CreatorID: accountID, UserID: managerID, Role: RoleManager,
		}, targetRole: RoleEditor, newRole: RoleManager, wantError: ErrForbidden},
		{name: "owner cannot be changed", actor: Member{
			CreatorID: accountID, UserID: ownerID, Role: RoleOwner,
		}, targetRole: RoleOwner, newRole: RoleManager, wantError: ErrOwnerProtected},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := newMembershipStore(test.actor, Member{
				CreatorID: accountID, UserID: targetID, Role: test.targetRole,
			})
			updated, err := NewMemberService(store, &userDirectoryStub{}).ChangeRole(
				context.Background(), test.actor.UserID, accountID, targetID, test.newRole,
			)
			if !errors.Is(err, test.wantError) {
				t.Fatalf("ChangeRole() error = %v, want %v", err, test.wantError)
			}
			if test.wantError != nil {
				if store.updateCalls != 0 {
					t.Fatalf("UpdateMemberRole() calls = %d", store.updateCalls)
				}
				return
			}
			if updated.Role != test.newRole || store.expectedRole != test.targetRole {
				t.Fatalf("ChangeRole() = %+v, expected role = %q", updated, store.expectedRole)
			}
		})
	}
}

func TestMemberServiceProtectsRoleChangeAndRemovalFromSelf(t *testing.T) {
	accountID := uuid.New()
	managerID := uuid.New()
	store := newMembershipStore(Member{
		CreatorID: accountID, UserID: managerID, Role: RoleManager,
	})
	service := NewMemberService(store, &userDirectoryStub{})

	if _, err := service.ChangeRole(
		context.Background(), managerID, accountID, managerID, RoleEditor,
	); !errors.Is(err, ErrForbidden) {
		t.Fatalf("ChangeRole(self) error = %v, want %v", err, ErrForbidden)
	}
	if err := service.Remove(
		context.Background(), managerID, accountID, managerID,
	); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Remove(self) error = %v, want %v", err, ErrForbidden)
	}
}

func TestMemberServiceRemovesOnlyManagedRoles(t *testing.T) {
	accountID := uuid.New()
	ownerID := uuid.New()
	managerID := uuid.New()
	targetID := uuid.New()
	tests := []struct {
		name       string
		actor      Member
		targetRole MemberRole
		wantError  error
	}{
		{name: "owner removes manager", actor: Member{
			CreatorID: accountID, UserID: ownerID, Role: RoleOwner,
		}, targetRole: RoleManager},
		{name: "manager removes uploader", actor: Member{
			CreatorID: accountID, UserID: managerID, Role: RoleManager,
		}, targetRole: RoleUploader},
		{name: "manager cannot remove manager", actor: Member{
			CreatorID: accountID, UserID: managerID, Role: RoleManager,
		}, targetRole: RoleManager, wantError: ErrForbidden},
		{name: "owner cannot be removed", actor: Member{
			CreatorID: accountID, UserID: ownerID, Role: RoleOwner,
		}, targetRole: RoleOwner, wantError: ErrOwnerProtected},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := newMembershipStore(test.actor, Member{
				CreatorID: accountID, UserID: targetID, Role: test.targetRole,
			})
			err := NewMemberService(store, &userDirectoryStub{}).Remove(
				context.Background(), test.actor.UserID, accountID, targetID,
			)
			if !errors.Is(err, test.wantError) {
				t.Fatalf("Remove() error = %v, want %v", err, test.wantError)
			}
			if test.wantError == nil &&
				(store.removeCalls != 1 || store.expectedRole != test.targetRole) {
				t.Fatalf("RemoveMember() calls = %d, expected role = %q",
					store.removeCalls, store.expectedRole)
			}
		})
	}
}

func TestMemberServiceListsMembersForOwnerAndManager(t *testing.T) {
	accountID := uuid.New()
	for _, role := range []MemberRole{RoleOwner, RoleManager} {
		t.Run(string(role), func(t *testing.T) {
			actorID := uuid.New()
			actor := Member{CreatorID: accountID, UserID: actorID, Role: role}
			store := newMembershipStore(actor, Member{
				CreatorID: accountID, UserID: uuid.New(), Role: RoleEditor,
			})
			members, err := NewMemberService(store, &userDirectoryStub{}).List(
				context.Background(), actorID, accountID,
			)
			if err != nil || len(members) != 2 {
				t.Fatalf("List() = %+v, %v", members, err)
			}
		})
	}
}

func TestMemberServiceAuthorizesOnlyScopedContentActions(t *testing.T) {
	accountID := uuid.New()
	tests := []struct {
		name       string
		membership *Member
		permission Permission
		wantError  error
	}{
		{name: "editor edits", membership: &Member{Role: RoleEditor}, permission: PermissionContentEdit},
		{name: "editor cannot upload", membership: &Member{Role: RoleEditor},
			permission: PermissionContentUpload, wantError: ErrForbidden},
		{name: "uploader uploads", membership: &Member{Role: RoleUploader},
			permission: PermissionContentUpload},
		{name: "uploader cannot publish", membership: &Member{Role: RoleUploader},
			permission: PermissionContentPublish, wantError: ErrForbidden},
		{name: "listener has no membership", permission: PermissionContentEdit,
			wantError: ErrForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actorID := uuid.New()
			var members []Member
			if test.membership != nil {
				membership := *test.membership
				membership.CreatorID = accountID
				membership.UserID = actorID
				members = append(members, membership)
			}
			store := newMembershipStore(members...)
			err := NewMemberService(store, &userDirectoryStub{}).Authorize(
				context.Background(), actorID, accountID, test.permission,
			)
			if !errors.Is(err, test.wantError) {
				t.Fatalf("Authorize() error = %v, want %v", err, test.wantError)
			}
		})
	}
}

type membershipStoreStub struct {
	members      map[uuid.UUID]Member
	addCalls     int
	updateCalls  int
	removeCalls  int
	expectedRole MemberRole
}

func newMembershipStore(members ...Member) *membershipStoreStub {
	store := &membershipStoreStub{members: make(map[uuid.UUID]Member, len(members))}
	for _, member := range members {
		store.members[member.UserID] = member
	}
	return store
}

func (store *membershipStoreStub) Member(
	_ context.Context,
	creatorID uuid.UUID,
	userID uuid.UUID,
) (Member, error) {
	member, exists := store.members[userID]
	if !exists || member.CreatorID != creatorID {
		return Member{}, ErrNotFound
	}
	return member, nil
}

func (store *membershipStoreStub) ListMembers(
	_ context.Context,
	creatorID uuid.UUID,
) ([]Member, error) {
	members := make([]Member, 0, len(store.members))
	for _, member := range store.members {
		if member.CreatorID == creatorID {
			members = append(members, member)
		}
	}
	return members, nil
}

func (store *membershipStoreStub) AddMember(
	_ context.Context,
	member Member,
) (Member, error) {
	store.addCalls++
	if _, exists := store.members[member.UserID]; exists {
		return Member{}, ErrMemberExists
	}
	store.members[member.UserID] = member
	return member, nil
}

func (store *membershipStoreStub) UpdateMemberRole(
	_ context.Context,
	creatorID uuid.UUID,
	userID uuid.UUID,
	expected MemberRole,
	target MemberRole,
) (Member, error) {
	store.updateCalls++
	store.expectedRole = expected
	member, exists := store.members[userID]
	if !exists || member.CreatorID != creatorID || member.Role != expected {
		return Member{}, ErrConflict
	}
	member.Role = target
	store.members[userID] = member
	return member, nil
}

func (store *membershipStoreStub) RemoveMember(
	_ context.Context,
	creatorID uuid.UUID,
	userID uuid.UUID,
	expected MemberRole,
) error {
	store.removeCalls++
	store.expectedRole = expected
	member, exists := store.members[userID]
	if !exists || member.CreatorID != creatorID || member.Role != expected {
		return ErrConflict
	}
	delete(store.members, userID)
	return nil
}

type userDirectoryStub struct {
	userID uuid.UUID
	err    error
}

func (stub *userDirectoryStub) RequireUser(_ context.Context, userID uuid.UUID) error {
	stub.userID = userID
	return stub.err
}

var (
	_ MembershipStore = (*membershipStoreStub)(nil)
	_ UserDirectory   = (*userDirectoryStub)(nil)
)
