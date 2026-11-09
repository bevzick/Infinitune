//go:build integration

package postgres

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCreatorRepositoryCreatesAccountAndOwnerAtomically(t *testing.T) {
	pool := openIntegrationDatabase(t)
	repository := NewCreatorRepository(pool)
	ctx := context.Background()
	accountID := uuid.New()
	ownerID := uuid.New()
	t.Cleanup(func() { cleanupCreatorAccount(t, pool, accountID) })

	created, err := repository.Create(ctx, creator.Account{
		ID: accountID, Type: creator.TypeArtist, DisplayName: "Integration Artist",
		VerificationStatus: creator.VerificationUnverified,
	}, creator.Member{
		CreatorID: accountID, UserID: ownerID, Role: creator.RoleOwner,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID != accountID || created.Type != creator.TypeArtist ||
		created.VerificationStatus != creator.VerificationUnverified ||
		created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatalf("Create() = %+v", created)
	}
	membership, err := repository.Member(ctx, accountID, ownerID)
	if err != nil {
		t.Fatalf("Member() error = %v", err)
	}
	if membership.Role != creator.RoleOwner || membership.CreatedAt.IsZero() {
		t.Fatalf("Member() = %+v", membership)
	}
	loaded, err := repository.ByID(ctx, accountID)
	if err != nil || loaded.DisplayName != "Integration Artist" {
		t.Fatalf("ByID() = %+v, %v", loaded, err)
	}
	accounts, err := repository.ListForUser(ctx, ownerID)
	if err != nil || len(accounts) != 1 || accounts[0].ID != accountID {
		t.Fatalf("ListForUser() = %+v, %v", accounts, err)
	}
}

func TestCreatorRepositoryRollsBackAccountWithoutOwner(t *testing.T) {
	pool := openIntegrationDatabase(t)
	repository := NewCreatorRepository(pool)
	ctx := context.Background()
	accountID := uuid.New()
	t.Cleanup(func() { cleanupCreatorAccount(t, pool, accountID) })

	_, err := repository.Create(ctx, creator.Account{
		ID: accountID, Type: creator.TypeLabel, DisplayName: "Rolled Back Label",
		VerificationStatus: creator.VerificationUnverified,
	}, creator.Member{
		CreatorID: uuid.New(), UserID: uuid.New(), Role: creator.RoleOwner,
	})
	if err == nil {
		t.Fatal("Create() error = nil")
	}
	if _, err = repository.ByID(ctx, accountID); !errors.Is(err, creator.ErrNotFound) {
		t.Fatalf("ByID() error = %v, want %v", err, creator.ErrNotFound)
	}
}

func TestCreatorRepositoryUpdatesAndTransitionsWithConcurrencyChecks(t *testing.T) {
	pool := openIntegrationDatabase(t)
	repository := NewCreatorRepository(pool)
	ctx := context.Background()
	accountID := uuid.New()
	t.Cleanup(func() { cleanupCreatorAccount(t, pool, accountID) })

	created, err := repository.Create(ctx, creator.Account{
		ID: accountID, Type: creator.TypeDistributor, DisplayName: "Original Distributor",
		VerificationStatus: creator.VerificationUnverified,
	}, creator.Member{
		CreatorID: accountID, UserID: uuid.New(), Role: creator.RoleOwner,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	update := created
	update.DisplayName = "Updated Distributor"
	updated, err := repository.Update(ctx, update, created.UpdatedAt)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.DisplayName != "Updated Distributor" ||
		!updated.UpdatedAt.After(created.UpdatedAt) {
		t.Fatalf("Update() = %+v, created = %+v", updated, created)
	}
	if _, err = repository.Update(ctx, update, created.UpdatedAt); !errors.Is(err, creator.ErrConflict) {
		t.Fatalf("Update(stale) error = %v, want %v", err, creator.ErrConflict)
	}

	pending, err := repository.TransitionVerification(
		ctx, accountID, creator.VerificationUnverified, creator.VerificationPending,
	)
	if err != nil {
		t.Fatalf("TransitionVerification() error = %v", err)
	}
	if pending.VerificationStatus != creator.VerificationPending {
		t.Fatalf("TransitionVerification() = %+v", pending)
	}
	if _, err = repository.TransitionVerification(
		ctx, accountID, creator.VerificationUnverified, creator.VerificationVerified,
	); !errors.Is(err, creator.ErrConflict) {
		t.Fatalf("TransitionVerification(stale) error = %v, want %v", err, creator.ErrConflict)
	}
}

func TestCreatorVerificationWorkflowIntegration(t *testing.T) {
	pool := openIntegrationDatabase(t)
	repository := NewCreatorRepository(pool)
	service := creator.NewService(repository)
	ctx := context.Background()
	ownerID := uuid.New()

	account, err := service.Create(ctx, ownerID, creator.CreateInput{
		Type: creator.TypeArtist, DisplayName: "Workflow Artist",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() { cleanupCreatorAccount(t, pool, account.ID) })
	if err = service.RequireVerified(ctx, account.ID); !errors.Is(err, creator.ErrVerificationRequired) {
		t.Fatalf("RequireVerified(unverified) error = %v, want %v",
			err, creator.ErrVerificationRequired)
	}
	pending, err := service.SubmitVerification(ctx, ownerID, account.ID)
	if err != nil || pending.VerificationStatus != creator.VerificationPending {
		t.Fatalf("SubmitVerification() = %+v, %v", pending, err)
	}
	verified, err := service.ReviewVerification(ctx, creator.Actor{
		UserID: uuid.New(), Roles: []string{creator.GlobalRoleModerator},
	}, account.ID, creator.VerificationVerified)
	if err != nil || verified.VerificationStatus != creator.VerificationVerified {
		t.Fatalf("ReviewVerification() = %+v, %v", verified, err)
	}
	if err = service.RequireVerified(ctx, account.ID); err != nil {
		t.Fatalf("RequireVerified(verified) error = %v", err)
	}
}

func TestCreatorRepositoryManagesMembersWithRoleConcurrency(t *testing.T) {
	pool := openIntegrationDatabase(t)
	repository := NewCreatorRepository(pool)
	ctx := context.Background()
	accountID := uuid.New()
	ownerID := uuid.New()
	memberID := uuid.New()
	t.Cleanup(func() { cleanupCreatorAccount(t, pool, accountID) })

	if _, err := repository.Create(ctx, creator.Account{
		ID: accountID, Type: creator.TypeLabel, DisplayName: "Member Repository Label",
		VerificationStatus: creator.VerificationUnverified,
	}, creator.Member{
		CreatorID: accountID, UserID: ownerID, Role: creator.RoleOwner,
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	added, err := repository.AddMember(ctx, creator.Member{
		CreatorID: accountID, UserID: memberID, Role: creator.RoleEditor,
	})
	if err != nil || added.Role != creator.RoleEditor || added.CreatedAt.IsZero() {
		t.Fatalf("AddMember() = %+v, %v", added, err)
	}
	if _, err = repository.AddMember(ctx, creator.Member{
		CreatorID: accountID, UserID: memberID, Role: creator.RoleUploader,
	}); !errors.Is(err, creator.ErrMemberExists) {
		t.Fatalf("AddMember(duplicate) error = %v, want %v", err, creator.ErrMemberExists)
	}
	members, err := repository.ListMembers(ctx, accountID)
	if err != nil || len(members) != 2 {
		t.Fatalf("ListMembers() = %+v, %v", members, err)
	}
	updated, err := repository.UpdateMemberRole(
		ctx, accountID, memberID, creator.RoleEditor, creator.RoleUploader,
	)
	if err != nil || updated.Role != creator.RoleUploader {
		t.Fatalf("UpdateMemberRole() = %+v, %v", updated, err)
	}
	if _, err = repository.UpdateMemberRole(
		ctx, accountID, memberID, creator.RoleEditor, creator.RoleManager,
	); !errors.Is(err, creator.ErrConflict) {
		t.Fatalf("UpdateMemberRole(stale) error = %v, want %v", err, creator.ErrConflict)
	}
	if err = repository.RemoveMember(
		ctx, accountID, memberID, creator.RoleUploader,
	); err != nil {
		t.Fatalf("RemoveMember() error = %v", err)
	}
	if err = repository.RemoveMember(
		ctx, accountID, memberID, creator.RoleUploader,
	); !errors.Is(err, creator.ErrConflict) {
		t.Fatalf("RemoveMember(stale) error = %v, want %v", err, creator.ErrConflict)
	}
}

func TestCreatorMemberPermissionWorkflowIntegration(t *testing.T) {
	pool := openIntegrationDatabase(t)
	repository := NewCreatorRepository(pool)
	members := creator.NewMemberService(repository, acceptingUserDirectory{})
	ctx := context.Background()
	accountID := uuid.New()
	ownerID := uuid.New()
	managerID := uuid.New()
	uploaderID := uuid.New()
	t.Cleanup(func() { cleanupCreatorAccount(t, pool, accountID) })

	if _, err := repository.Create(ctx, creator.Account{
		ID: accountID, Type: creator.TypeDistributor, DisplayName: "Permission Distributor",
		VerificationStatus: creator.VerificationUnverified,
	}, creator.Member{
		CreatorID: accountID, UserID: ownerID, Role: creator.RoleOwner,
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := members.Add(
		ctx, ownerID, accountID, managerID, creator.RoleManager,
	); err != nil {
		t.Fatalf("owner Add(manager) error = %v", err)
	}
	if _, err := members.Add(
		ctx, managerID, accountID, uploaderID, creator.RoleUploader,
	); err != nil {
		t.Fatalf("manager Add(uploader) error = %v", err)
	}
	if err := members.Authorize(
		ctx, uploaderID, accountID, creator.PermissionContentUpload,
	); err != nil {
		t.Fatalf("Authorize(upload) error = %v", err)
	}
	if err := members.Authorize(
		ctx, uploaderID, accountID, creator.PermissionContentPublish,
	); !errors.Is(err, creator.ErrForbidden) {
		t.Fatalf("Authorize(publish) error = %v, want %v", err, creator.ErrForbidden)
	}
	if _, err := members.Add(
		ctx, managerID, accountID, uuid.New(), creator.RoleManager,
	); !errors.Is(err, creator.ErrForbidden) {
		t.Fatalf("manager Add(manager) error = %v, want %v", err, creator.ErrForbidden)
	}
}

type acceptingUserDirectory struct{}

func (acceptingUserDirectory) RequireUser(context.Context, uuid.UUID) error { return nil }

func openIntegrationDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if os.Getenv("INFINITUNE_INTEGRATION_TESTS") != "1" {
		t.Skip("set INFINITUNE_INTEGRATION_TESTS=1 to run integration tests")
	}
	port, err := strconv.ParseUint(envOrDefault("POSTGRES_PORT", "5432"), 10, 16)
	if err != nil {
		t.Fatalf("parse POSTGRES_PORT: %v", err)
	}
	config, err := pgxpool.ParseConfig("sslmode=disable")
	if err != nil {
		t.Fatalf("parse postgres config: %v", err)
	}
	config.ConnConfig.Host = envOrDefault("POSTGRES_HOST", "127.0.0.1")
	config.ConnConfig.Port = uint16(port)
	config.ConnConfig.User = requiredEnv(t, "POSTGRES_USER")
	config.ConnConfig.Password = requiredEnv(t, "POSTGRES_PASSWORD")
	config.ConnConfig.Database = requiredEnv(t, "POSTGRES_CONTENT_DB")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping postgres: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func cleanupCreatorAccount(t *testing.T, pool *pgxpool.Pool, accountID uuid.UUID) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := pool.Exec(ctx, `
		DELETE FROM content.creator_members WHERE creator_id = $1
	`, accountID); err != nil {
		t.Errorf("cleanup creator members: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		DELETE FROM content.creator_accounts WHERE id = $1
	`, accountID); err != nil {
		t.Errorf("cleanup creator account: %v", err)
	}
}

func requiredEnv(t *testing.T, key string) string {
	t.Helper()
	value := os.Getenv(key)
	if value == "" {
		t.Fatalf("%s is required", key)
	}
	return value
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
