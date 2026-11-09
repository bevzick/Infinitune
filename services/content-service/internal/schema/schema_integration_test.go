//go:build integration

package schema

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestContentSchemaCreatesAccountWithCrossServiceOwner(t *testing.T) {
	tx := beginIntegrationTransaction(t)
	ctx := context.Background()
	creatorID := uuid.New()
	userID := uuid.New()

	_, err := tx.Exec(ctx, `
		INSERT INTO content.creator_accounts (id, account_type, display_name)
		VALUES ($1, 'artist', 'Integration Artist')
	`, creatorID)
	if err != nil {
		t.Fatalf("insert creator account: %v", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO content.creator_members (creator_id, user_id, member_role)
		VALUES ($1, $2, 'owner')
	`, creatorID, userID)
	if err != nil {
		t.Fatalf("insert member with cross-service user UUID: %v", err)
	}

	var accountType, status, role string
	var createdAt, updatedAt time.Time
	if err = tx.QueryRow(ctx, `
		SELECT account.account_type, account.verification_status,
		       member.member_role, account.created_at, account.updated_at
		FROM content.creator_accounts AS account
		JOIN content.creator_members AS member ON member.creator_id = account.id
		WHERE account.id = $1 AND member.user_id = $2
	`, creatorID, userID).Scan(&accountType, &status, &role, &createdAt, &updatedAt); err != nil {
		t.Fatalf("read creator account and owner: %v", err)
	}
	if accountType != "artist" || status != "unverified" || role != "owner" {
		t.Fatalf("account defaults = type %q, status %q, role %q", accountType, status, role)
	}
	if createdAt.IsZero() || updatedAt.IsZero() {
		t.Fatalf("account timestamps = created %v, updated %v", createdAt, updatedAt)
	}
}

func TestContentSchemaSupportsDeclaredCreatorTypesAndVerificationStatuses(t *testing.T) {
	tx := beginIntegrationTransaction(t)
	ctx := context.Background()
	creatorTypes := []string{"artist", "label", "distributor"}
	statuses := []string{"unverified", "pending", "verified", "rejected"}

	for _, creatorType := range creatorTypes {
		for _, status := range statuses {
			if _, err := tx.Exec(ctx, `
				INSERT INTO content.creator_accounts (
					id, account_type, display_name, verification_status
				)
				VALUES ($1, $2, $3, $4)
			`, uuid.New(), creatorType, creatorType+" "+status, status); err != nil {
				t.Fatalf("insert creator type %q with status %q: %v", creatorType, status, err)
			}
		}
	}
}

func TestContentSchemaRejectsInvalidAccountsAndMemberships(t *testing.T) {
	tests := []struct {
		name string
		run  func(context.Context, pgx.Tx) error
	}{
		{
			name: "blank display name",
			run: func(ctx context.Context, tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `
					INSERT INTO content.creator_accounts (id, account_type, display_name)
					VALUES ($1, 'artist', '   ')
				`, uuid.New())
				return err
			},
		},
		{
			name: "unsupported creator type",
			run: func(ctx context.Context, tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `
					INSERT INTO content.creator_accounts (id, account_type, display_name)
					VALUES ($1, 'listener', 'Invalid Type')
				`, uuid.New())
				return err
			},
		},
		{
			name: "unsupported verification status",
			run: func(ctx context.Context, tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `
					INSERT INTO content.creator_accounts (
						id, account_type, display_name, verification_status
					)
					VALUES ($1, 'label', 'Invalid Status', 'approved')
				`, uuid.New())
				return err
			},
		},
		{
			name: "member without creator account",
			run: func(ctx context.Context, tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `
					INSERT INTO content.creator_members (creator_id, user_id, member_role)
					VALUES ($1, $2, 'owner')
				`, uuid.New(), uuid.New())
				return err
			},
		},
		{
			name: "unsupported member role",
			run: func(ctx context.Context, tx pgx.Tx) error {
				creatorID := uuid.New()
				if _, err := tx.Exec(ctx, `
					INSERT INTO content.creator_accounts (id, account_type, display_name)
					VALUES ($1, 'distributor', 'Role Check')
				`, creatorID); err != nil {
					return err
				}
				_, err := tx.Exec(ctx, `
					INSERT INTO content.creator_members (creator_id, user_id, member_role)
					VALUES ($1, $2, 'administrator')
				`, creatorID, uuid.New())
				return err
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tx := beginIntegrationTransaction(t)
			if err := test.run(context.Background(), tx); err == nil {
				t.Fatal("constraint accepted invalid data")
			}
		})
	}
}

func TestCreatorMembershipCompositeKeyPreventsDuplicates(t *testing.T) {
	tx := beginIntegrationTransaction(t)
	ctx := context.Background()
	creatorID := uuid.New()
	userID := uuid.New()
	_, err := tx.Exec(ctx, `
		INSERT INTO content.creator_accounts (id, account_type, display_name)
		VALUES ($1, 'label', 'No Duplicate Members')
	`, creatorID)
	if err != nil {
		t.Fatalf("insert creator account: %v", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO content.creator_members (creator_id, user_id, member_role)
		VALUES ($1, $2, 'owner')
	`, creatorID, userID)
	if err != nil {
		t.Fatalf("insert first creator member: %v", err)
	}
	if _, err = tx.Exec(ctx, "SAVEPOINT duplicate_member"); err != nil {
		t.Fatalf("create savepoint: %v", err)
	}
	_, duplicateErr := tx.Exec(ctx, `
		INSERT INTO content.creator_members (creator_id, user_id, member_role)
		VALUES ($1, $2, 'manager')
	`, creatorID, userID)
	if duplicateErr == nil {
		t.Fatal("composite primary key accepted duplicate creator member")
	}
	if _, err = tx.Exec(ctx, "ROLLBACK TO SAVEPOINT duplicate_member"); err != nil {
		t.Fatalf("rollback duplicate insert: %v", err)
	}
	var count int
	if err = tx.QueryRow(ctx, `
		SELECT count(*) FROM content.creator_members
		WHERE creator_id = $1 AND user_id = $2
	`, creatorID, userID).Scan(&count); err != nil {
		t.Fatalf("count creator members: %v", err)
	}
	if count != 1 {
		t.Fatalf("creator member count = %d, want 1", count)
	}
}

func beginIntegrationTransaction(t *testing.T) pgx.Tx {
	t.Helper()
	pool := openIntegrationDatabase(t)
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(ctx)
	})
	return tx
}

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
