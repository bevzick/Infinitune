//go:build integration

package schema

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCommentSchemaCreatesRootAndReplyForCrossServiceIdentities(t *testing.T) {
	tx := beginIntegrationTransaction(t)
	ctx := context.Background()
	trackID, rootID, replyID := uuid.New(), uuid.New(), uuid.New()
	rootUserID, replyUserID := uuid.New(), uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO comment.comments (id, user_id, track_id, content)
		VALUES ($1, $2, $3, 'Root comment')
	`, rootID, rootUserID, trackID); err != nil {
		t.Fatalf("insert root comment: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO comment.comments (id, user_id, track_id, parent_id, content)
		VALUES ($1, $2, $3, $4, 'Direct reply')
	`, replyID, replyUserID, trackID, rootID); err != nil {
		t.Fatalf("insert direct reply: %v", err)
	}

	var status string
	var createdAt, updatedAt time.Time
	if err := tx.QueryRow(ctx, `
		SELECT status, created_at, updated_at
		FROM comment.comments WHERE id = $1
	`, replyID).Scan(&status, &createdAt, &updatedAt); err != nil {
		t.Fatalf("read reply defaults: %v", err)
	}
	if status != "published" || createdAt.IsZero() || updatedAt.IsZero() {
		t.Fatalf("reply defaults = status %q, created %v, updated %v", status, createdAt, updatedAt)
	}
}

func TestCommentSchemaRejectsNestedAndCrossTrackReplies(t *testing.T) {
	tests := []struct {
		name string
		run  func(context.Context, pgx.Tx) error
	}{
		{name: "nested reply", run: func(ctx context.Context, tx pgx.Tx) error {
			trackID, rootID, replyID := uuid.New(), uuid.New(), uuid.New()
			if _, err := tx.Exec(ctx, `
				INSERT INTO comment.comments (id, user_id, track_id, content)
				VALUES ($1, $2, $3, 'Root')
			`, rootID, uuid.New(), trackID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO comment.comments (id, user_id, track_id, parent_id, content)
				VALUES ($1, $2, $3, $4, 'Reply')
			`, replyID, uuid.New(), trackID, rootID); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `
				INSERT INTO comment.comments (id, user_id, track_id, parent_id, content)
				VALUES ($1, $2, $3, $4, 'Nested reply')
			`, uuid.New(), uuid.New(), trackID, replyID)
			return err
		}},
		{name: "parent from another track", run: func(ctx context.Context, tx pgx.Tx) error {
			rootID := uuid.New()
			if _, err := tx.Exec(ctx, `
				INSERT INTO comment.comments (id, user_id, track_id, content)
				VALUES ($1, $2, $3, 'First track')
			`, rootID, uuid.New(), uuid.New()); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `
				INSERT INTO comment.comments (id, user_id, track_id, parent_id, content)
				VALUES ($1, $2, $3, $4, 'Wrong track')
			`, uuid.New(), uuid.New(), uuid.New(), rootID)
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tx := beginIntegrationTransaction(t)
			if err := test.run(context.Background(), tx); err == nil {
				t.Fatal("tree constraints accepted invalid reply")
			}
		})
	}
}

func TestCommentSchemaRejectsInvalidContentAndDeletionState(t *testing.T) {
	tests := []struct {
		name    string
		content string
		status  string
		deleted *time.Time
	}{
		{name: "blank content", content: "   ", status: "published"},
		{name: "content too long", content: strings.Repeat("x", 2001), status: "published"},
		{name: "deleted without timestamp", content: "Deleted", status: "deleted"},
		{name: "published with deletion timestamp", content: "Published", status: "published",
			deleted: timePointer(time.Now().UTC())},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tx := beginIntegrationTransaction(t)
			_, err := tx.Exec(context.Background(), `
				INSERT INTO comment.comments (
					id, user_id, track_id, content, status, deleted_at
				) VALUES ($1, $2, $3, $4, $5, $6)
			`, uuid.New(), uuid.New(), uuid.New(), test.content, test.status, test.deleted)
			if err == nil {
				t.Fatal("comment constraints accepted invalid state")
			}
		})
	}
}

func TestCommentReportReferencesLocalCommentAndUsesPendingStatus(t *testing.T) {
	tx := beginIntegrationTransaction(t)
	ctx := context.Background()
	commentID, reportID := uuid.New(), uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO comment.comments (id, user_id, track_id, content)
		VALUES ($1, $2, $3, 'Reportable comment')
	`, commentID, uuid.New(), uuid.New()); err != nil {
		t.Fatalf("insert comment: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO comment.comment_reports (
			id, comment_id, reporter_user_id, reason, description
		) VALUES ($1, $2, $3, 'harassment', 'Report details')
	`, reportID, commentID, uuid.New()); err != nil {
		t.Fatalf("insert report: %v", err)
	}
	var status string
	var updatedAt *time.Time
	if err := tx.QueryRow(ctx, `
		SELECT status, updated_at FROM comment.comment_reports WHERE id = $1
	`, reportID).Scan(&status, &updatedAt); err != nil {
		t.Fatalf("read report defaults: %v", err)
	}
	if status != "pending" || updatedAt != nil {
		t.Fatalf("report defaults = status %q, updated_at %v", status, updatedAt)
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
	config.ConnConfig.Database = requiredEnv(t, "POSTGRES_COMMENT_DB")
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

func timePointer(value time.Time) *time.Time { return &value }
