//go:build integration

package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/auth-service/internal/auth"
	"github.com/bevzick/Infinitune/services/auth-service/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const integrationTimeout = 10 * time.Second

func TestRegistrationAndLoginRepositories(t *testing.T) {
	pool := openIntegrationPostgres(t)
	userID := uuid.New()
	duplicateUserID := uuid.New()
	cleanupIntegrationUsers(t, pool, userID, duplicateUserID)

	now := time.Now().UTC().Truncate(time.Microsecond)
	email := fmt.Sprintf("integration-%s@example.invalid", userID)
	registration := NewRegistrationRepository(pool)
	ctx, cancel := context.WithTimeout(context.Background(), integrationTimeout)
	defer cancel()

	err := registration.CreateUserWithRole(ctx, auth.CreateUserWithRoleParams{
		User: auth.User{
			ID:            userID,
			Email:         email,
			PasswordHash:  "integration-password-hash",
			Status:        auth.AccountStatusActive,
			EmailVerified: false,
			CreatedAt:     now,
			UpdatedAt:     now,
		},
		RoleName: auth.RoleListener,
	})
	if err != nil {
		t.Fatalf("CreateUserWithRole() error = %v", err)
	}

	login := NewLoginRepository(pool)
	user, err := login.FindUserByEmail(ctx, email)
	if err != nil {
		t.Fatalf("FindUserByEmail() error = %v", err)
	}
	if user.ID != userID || user.Email != email || user.PasswordHash != "integration-password-hash" {
		t.Fatalf("FindUserByEmail() = %+v", user)
	}
	roles, err := login.ListRoleNamesForUser(ctx, userID)
	if err != nil {
		t.Fatalf("ListRoleNamesForUser() error = %v", err)
	}
	if len(roles) != 1 || roles[0] != auth.RoleListener {
		t.Fatalf("ListRoleNamesForUser() = %v", roles)
	}

	loggedInAt := now.Add(time.Minute)
	if err = login.RecordLogin(ctx, userID, loggedInAt); err != nil {
		t.Fatalf("RecordLogin() error = %v", err)
	}
	user, err = login.FindUserByID(ctx, userID)
	if err != nil {
		t.Fatalf("FindUserByID() error = %v", err)
	}
	if user.LastLoginAt == nil || !user.LastLoginAt.Equal(loggedInAt) {
		t.Fatalf("FindUserByID() last login = %v, want %v", user.LastLoginAt, loggedInAt)
	}

	err = registration.CreateUserWithRole(ctx, auth.CreateUserWithRoleParams{
		User: auth.User{
			ID:           duplicateUserID,
			Email:        email,
			PasswordHash: "another-hash",
			Status:       auth.AccountStatusActive,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		RoleName: auth.RoleListener,
	})
	if !errors.Is(err, auth.ErrEmailAlreadyRegistered) {
		t.Fatalf("CreateUserWithRole(duplicate) error = %v", err)
	}
}

func TestUserAndRoleAssignmentRepositories(t *testing.T) {
	pool := openIntegrationPostgres(t)
	adminID := uuid.New()
	targetID := uuid.New()
	cleanupIntegrationUsers(t, pool, adminID, targetID)
	ctx, cancel := context.WithTimeout(context.Background(), integrationTimeout)
	defer cancel()
	now := time.Now().UTC().Truncate(time.Microsecond)

	users := NewUserRepository(pool)
	for _, userID := range []uuid.UUID{adminID, targetID} {
		_, err := users.Create(ctx, CreateUserParams{
			ID:            userID,
			Email:         fmt.Sprintf("integration-%s@example.invalid", userID),
			PasswordHash:  "integration-password-hash",
			Status:        auth.AccountStatusActive,
			EmailVerified: false,
			CreatedAt:     now,
			UpdatedAt:     now,
		})
		if err != nil {
			t.Fatalf("Create(%v) error = %v", userID, err)
		}
	}

	roles := NewRoleRepository(pool)
	listenerRole := integrationRoleByName(t, ctx, roles, auth.RoleListener)
	adminRole := integrationRoleByName(t, ctx, roles, auth.RoleAdmin)
	if err := roles.AssignToUser(ctx, adminID, listenerRole.ID, now); err != nil {
		t.Fatalf("AssignToUser(listener admin) error = %v", err)
	}
	if err := roles.AssignToUser(ctx, adminID, adminRole.ID, now); err != nil {
		t.Fatalf("AssignToUser(admin) error = %v", err)
	}
	if err := roles.AssignToUser(ctx, targetID, listenerRole.ID, now); err != nil {
		t.Fatalf("AssignToUser(listener target) error = %v", err)
	}

	assignments := NewRoleAssignmentRepository(pool)
	if err := assignments.AssignRoleToUser(ctx, targetID, auth.RoleCreator, now); err != nil {
		t.Fatalf("AssignRoleToUser() error = %v", err)
	}
	if err := assignments.AssignRoleToUser(ctx, targetID, auth.RoleCreator, now); !errors.Is(err, auth.ErrRoleAlreadyAssigned) {
		t.Fatalf("AssignRoleToUser(duplicate) error = %v", err)
	}
	assignedRoles, err := assignments.ListRoleNamesForUser(ctx, targetID)
	if err != nil {
		t.Fatalf("ListRoleNamesForUser() error = %v", err)
	}
	if len(assignedRoles) != 2 || assignedRoles[0] != auth.RoleListener || assignedRoles[1] != auth.RoleCreator {
		t.Fatalf("ListRoleNamesForUser() = %v", assignedRoles)
	}

	if err = assignments.RemoveRoleFromUser(ctx, targetID, auth.RoleCreator); err != nil {
		t.Fatalf("RemoveRoleFromUser() error = %v", err)
	}
	if err = assignments.RemoveRoleFromUser(ctx, targetID, auth.RoleCreator); !errors.Is(err, auth.ErrRoleNotAssigned) {
		t.Fatalf("RemoveRoleFromUser(second) error = %v", err)
	}
	if err = assignments.AssignRoleToUser(ctx, targetID, auth.RoleAdmin, now); err != nil {
		t.Fatalf("AssignRoleToUser(second admin) error = %v", err)
	}
	if err = assignments.RemoveRoleFromUser(ctx, targetID, auth.RoleAdmin); err != nil {
		t.Fatalf("RemoveRoleFromUser(second admin) error = %v", err)
	}
	var otherAdminCount int
	err = pool.QueryRow(ctx, `
		SELECT count(*)
		FROM auth.user_roles AS ur
		JOIN auth.roles AS r ON r.id = ur.role_id
		WHERE r.name = $1 AND ur.user_id <> $2`, auth.RoleAdmin, adminID).Scan(&otherAdminCount)
	if err != nil {
		t.Fatalf("count existing admins: %v", err)
	}
	if otherAdminCount == 0 {
		if err = assignments.RemoveRoleFromUser(ctx, adminID, auth.RoleAdmin); !errors.Is(err, auth.ErrCannotRemoveLastAdmin) {
			t.Fatalf("RemoveRoleFromUser(last admin) error = %v", err)
		}
	}
	activeStatus := auth.AccountStatusActive
	page, err := assignments.ListUsers(ctx, auth.UserPageRequest{
		Status: &activeStatus, Limit: 100,
	})
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	foundTarget := false
	for _, item := range page.Items {
		if item.User.ID == targetID {
			foundTarget = true
			if len(item.Roles) != 1 || item.Roles[0] != auth.RoleListener {
				t.Fatalf("ListUsers() target roles = %v", item.Roles)
			}
		}
	}
	if !foundTarget {
		t.Fatalf("ListUsers() did not include target %s", targetID)
	}
	blocked, err := assignments.UpdateUserStatus(
		ctx, targetID, auth.AccountStatusBlocked, now.Add(time.Minute),
	)
	if err != nil || blocked.Status != auth.AccountStatusBlocked {
		t.Fatalf("UpdateUserStatus(blocked) = %+v, %v", blocked, err)
	}
	if _, err = assignments.UpdateUserStatus(
		ctx, targetID, auth.AccountStatusActive, now.Add(2*time.Minute),
	); err != nil {
		t.Fatalf("UpdateUserStatus(active) error = %v", err)
	}

	if err = users.UpdateStatus(ctx, targetID, auth.AccountStatusBlocked, now.Add(3*time.Minute)); err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	if err = users.SetEmailVerified(ctx, targetID, true, now.Add(4*time.Minute)); err != nil {
		t.Fatalf("SetEmailVerified() error = %v", err)
	}
	updated, err := users.ByID(ctx, targetID)
	if err != nil {
		t.Fatalf("ByID() error = %v", err)
	}
	if updated.Status != auth.AccountStatusBlocked || !updated.EmailVerified {
		t.Fatalf("ByID() updated user = %+v", updated)
	}
}

func openIntegrationPostgres(t *testing.T) *pgxpool.Pool {
	t.Helper()
	requireIntegrationEnvironment(t)

	port, err := strconv.ParseUint(requiredIntegrationEnv(t, "POSTGRES_PORT"), 10, 16)
	if err != nil {
		t.Fatalf("parse POSTGRES_PORT: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), integrationTimeout)
	defer cancel()
	pool, err := database.Open(ctx, database.Config{
		Host:     requiredIntegrationEnv(t, "POSTGRES_HOST"),
		Port:     uint16(port),
		User:     requiredIntegrationEnv(t, "POSTGRES_USER"),
		Password: requiredIntegrationEnv(t, "POSTGRES_PASSWORD"),
		Name:     requiredIntegrationEnv(t, "POSTGRES_AUTH_DB"),
	})
	if err != nil {
		t.Fatalf("open integration postgres: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func cleanupIntegrationUsers(t *testing.T, pool *pgxpool.Pool, userIDs ...uuid.UUID) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), integrationTimeout)
		defer cancel()
		if _, err := pool.Exec(ctx, `DELETE FROM auth.user_roles WHERE user_id = ANY($1)`, userIDs); err != nil {
			t.Errorf("cleanup integration roles: %v", err)
		}
		if _, err := pool.Exec(ctx, `DELETE FROM auth.users_auth WHERE id = ANY($1)`, userIDs); err != nil {
			t.Errorf("cleanup integration users: %v", err)
		}
	})
}

func integrationRoleByName(
	t *testing.T,
	ctx context.Context,
	repository *RoleRepository,
	name string,
) auth.Role {
	t.Helper()
	role, err := repository.ByName(ctx, name)
	if err != nil {
		t.Fatalf("ByName(%q) error = %v; apply Auth migrations before integration tests", name, err)
	}
	return role
}

func requireIntegrationEnvironment(t *testing.T) {
	t.Helper()
	if os.Getenv("INFINITUNE_INTEGRATION_TESTS") != "1" {
		t.Skip("set INFINITUNE_INTEGRATION_TESTS=1 to run repository integration tests")
	}
}

func requiredIntegrationEnv(t *testing.T, key string) string {
	t.Helper()
	value := os.Getenv(key)
	if value == "" {
		t.Fatalf("%s is required for integration tests", key)
	}
	return value
}
