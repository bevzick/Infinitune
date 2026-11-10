//go:build integration

package postgres

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/comment-service/internal/comment"
	"github.com/bevzick/Infinitune/services/comment-service/internal/moderation"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCommentRepositoryManagesOwnedCommentsAndReplies(t *testing.T) {
	pool := openIntegrationDatabase(t)
	repository := NewCommentRepository(pool)
	ctx := context.Background()
	ownerID, replyUserID, strangerID := uuid.New(), uuid.New(), uuid.New()
	trackID, rootID, replyID := uuid.New(), uuid.New(), uuid.New()
	cleanupUsers(t, pool, ownerID, replyUserID, strangerID)

	root, err := repository.Create(ctx, comment.Comment{
		ID: rootID, UserID: ownerID, TrackID: trackID, Content: "Root",
	})
	if err != nil || root.Status != comment.StatusPublished || root.CreatedAt.IsZero() {
		t.Fatalf("Create(root) = %+v, %v", root, err)
	}
	reply, err := repository.Create(ctx, comment.Comment{
		ID: replyID, UserID: replyUserID, TrackID: trackID, ParentID: &rootID, Content: "Reply",
	})
	if err != nil || reply.ParentID == nil || *reply.ParentID != rootID {
		t.Fatalf("Create(reply) = %+v, %v", reply, err)
	}
	if _, err = repository.Create(ctx, comment.Comment{
		ID: uuid.New(), UserID: strangerID, TrackID: trackID, ParentID: &replyID, Content: "Nested",
	}); !errors.Is(err, comment.ErrParentInvalid) {
		t.Fatalf("Create(nested) error = %v, want %v", err, comment.ErrParentInvalid)
	}
	page, err := repository.List(ctx, trackID, comment.PageRequest{Limit: 10})
	if err != nil || page.Total != 2 || len(page.Items) != 2 {
		t.Fatalf("List() = %+v, %v", page, err)
	}
	visible, err := repository.ExistingVisibleIDs(ctx, []uuid.UUID{rootID, replyID, uuid.New()})
	if err != nil || len(visible) != 2 {
		t.Fatalf("ExistingVisibleIDs() = %v, %v", visible, err)
	}
	if _, err = repository.UpdateOwned(ctx, rootID, strangerID, "Stolen"); !errors.Is(err, comment.ErrForbidden) {
		t.Fatalf("UpdateOwned(stranger) error = %v", err)
	}
	updated, err := repository.UpdateOwned(ctx, rootID, ownerID, "Updated root")
	if err != nil || updated.Content != "Updated root" || updated.UserID != ownerID {
		t.Fatalf("UpdateOwned(owner) = %+v, %v", updated, err)
	}
	if err = repository.DeleteOwned(ctx, rootID, strangerID); !errors.Is(err, comment.ErrForbidden) {
		t.Fatalf("DeleteOwned(stranger) error = %v", err)
	}
	report, err := repository.CreateReport(ctx, comment.Report{
		ID: uuid.New(), CommentID: rootID, ReporterUserID: replyUserID,
		Reason: comment.ReportReasonHarassment, Description: "Personal attack",
	})
	if err != nil || report.Status != comment.ReportStatusPending || report.CreatedAt.IsZero() {
		t.Fatalf("CreateReport() = %+v, %v", report, err)
	}
	if _, err = repository.CreateReport(ctx, comment.Report{
		ID: uuid.New(), CommentID: rootID, ReporterUserID: replyUserID,
		Reason: comment.ReportReasonOther, Description: "Duplicate",
	}); !errors.Is(err, comment.ErrReportDuplicate) {
		t.Fatalf("CreateReport(duplicate) error = %v, want %v", err, comment.ErrReportDuplicate)
	}
	if _, err = repository.CreateReport(ctx, comment.Report{
		ID: uuid.New(), CommentID: rootID, ReporterUserID: ownerID,
		Reason: comment.ReportReasonOther, Description: "Own comment",
	}); !errors.Is(err, comment.ErrOwnCommentReport) {
		t.Fatalf("CreateReport(own) error = %v, want %v", err, comment.ErrOwnCommentReport)
	}
	for range 2 {
		if err = repository.DeleteOwned(ctx, rootID, ownerID); err != nil {
			t.Fatalf("DeleteOwned(owner) error = %v", err)
		}
	}
	if _, err = repository.UpdateOwned(ctx, rootID, ownerID, "Resurrected"); !errors.Is(err, comment.ErrNotEditable) {
		t.Fatalf("UpdateOwned(deleted) error = %v, want %v", err, comment.ErrNotEditable)
	}
	if _, err = repository.CreateReport(ctx, comment.Report{
		ID: uuid.New(), CommentID: rootID, ReporterUserID: strangerID,
		Reason: comment.ReportReasonOther, Description: "Deleted comment",
	}); !errors.Is(err, comment.ErrNotFound) {
		t.Fatalf("CreateReport(deleted) error = %v, want %v", err, comment.ErrNotFound)
	}
	page, err = repository.List(ctx, trackID, comment.PageRequest{Limit: 10})
	if err != nil || page.Total != 0 || len(page.Items) != 0 {
		t.Fatalf("List(after root delete) = %+v, %v", page, err)
	}
	visible, err = repository.ExistingVisibleIDs(ctx, []uuid.UUID{rootID, replyID})
	if err != nil || len(visible) != 0 {
		t.Fatalf("ExistingVisibleIDs(after root delete) = %v, %v", visible, err)
	}
}

func TestCommentRepositoryRateLimitsReporter(t *testing.T) {
	pool := openIntegrationDatabase(t)
	repository := NewCommentRepository(pool)
	ctx := context.Background()
	ownerID, reporterID, trackID := uuid.New(), uuid.New(), uuid.New()
	cleanupUsers(t, pool, ownerID, reporterID)
	for index := range maximumReportsPerHour + 1 {
		commentID := uuid.New()
		if _, err := pool.Exec(ctx, `
			INSERT INTO comment.comments (id, user_id, track_id, content)
			VALUES ($1, $2, $3, $4)
		`, commentID, ownerID, trackID, "Reported comment "+strconv.Itoa(index)); err != nil {
			t.Fatalf("insert comment %d: %v", index, err)
		}
		_, err := repository.CreateReport(ctx, comment.Report{
			ID: uuid.New(), CommentID: commentID, ReporterUserID: reporterID,
			Reason: comment.ReportReasonSpam, Description: "Repeated spam",
		})
		if index < maximumReportsPerHour && err != nil {
			t.Fatalf("CreateReport(%d) error = %v", index, err)
		}
		if index == maximumReportsPerHour && !errors.Is(err, comment.ErrReportRateLimited) {
			t.Fatalf("CreateReport(over limit) error = %v, want %v", err, comment.ErrReportRateLimited)
		}
	}
}

func TestCommentRepositoryModeratesReportWithAuditHistory(t *testing.T) {
	pool := openIntegrationDatabase(t)
	repository := NewCommentRepository(pool)
	ctx := context.Background()
	ownerID, reporterID, moderatorID := uuid.New(), uuid.New(), uuid.New()
	trackID, commentID, reportID := uuid.New(), uuid.New(), uuid.New()
	cleanupUsers(t, pool, ownerID, reporterID, moderatorID)
	if _, err := repository.Create(ctx, comment.Comment{
		ID: commentID, UserID: ownerID, TrackID: trackID, Content: "Moderate me",
	}); err != nil {
		t.Fatalf("create comment: %v", err)
	}
	if _, err := repository.CreateReport(ctx, comment.Report{
		ID: reportID, CommentID: commentID, ReporterUserID: reporterID,
		Reason: comment.ReportReasonHarassment, Description: "Abusive",
	}); err != nil {
		t.Fatalf("create report: %v", err)
	}
	status := comment.ReportStatusPending
	queue, err := repository.ListModerationCases(ctx, moderation.PageRequest{Status: &status, Limit: 10})
	if err != nil || queue.Total < 1 || len(queue.Items) < 1 {
		t.Fatalf("ListModerationCases() = %+v, %v", queue, err)
	}
	actions := []struct {
		action        moderation.Action
		reportStatus  comment.ReportStatus
		commentStatus comment.Status
	}{
		{moderation.ActionStartReview, comment.ReportStatusReviewing, comment.StatusPublished},
		{moderation.ActionHideComment, comment.ReportStatusReviewing, comment.StatusHidden},
		{moderation.ActionRestoreComment, comment.ReportStatusReviewing, comment.StatusPublished},
		{moderation.ActionResolveReport, comment.ReportStatusResolved, comment.StatusPublished},
	}
	for _, step := range actions {
		result, actionErr := repository.ApplyModerationAction(ctx, moderation.Decision{
			ID: uuid.New(), ReportID: reportID, ModeratorUserID: moderatorID,
			Action: step.action, Note: "Audit note",
		})
		if actionErr != nil || result.ResultingReportStatus != step.reportStatus ||
			result.ResultingCommentStatus != step.commentStatus || result.CreatedAt.IsZero() {
			t.Fatalf("ApplyModerationAction(%s) = %+v, %v", step.action, result, actionErr)
		}
	}
	if _, err = repository.ApplyModerationAction(ctx, moderation.Decision{
		ID: uuid.New(), ReportID: reportID, ModeratorUserID: moderatorID,
		Action: moderation.ActionRejectReport, Note: "Too late",
	}); !errors.Is(err, moderation.ErrConflict) {
		t.Fatalf("ApplyModerationAction(terminal) error = %v, want %v", err, moderation.ErrConflict)
	}
	var auditCount int
	if err = pool.QueryRow(ctx, `
		SELECT count(*) FROM comment.report_moderation_actions WHERE report_id = $1
	`, reportID).Scan(&auditCount); err != nil || auditCount != len(actions) {
		t.Fatalf("audit count = %d, %v", auditCount, err)
	}
}

func TestCommentRepositoryRateLimitsAuthor(t *testing.T) {
	pool := openIntegrationDatabase(t)
	repository := NewCommentRepository(pool)
	ctx := context.Background()
	userID, trackID := uuid.New(), uuid.New()
	cleanupUsers(t, pool, userID)
	for index := range maximumCommentsPerMinute {
		_, err := repository.Create(ctx, comment.Comment{
			ID: uuid.New(), UserID: userID, TrackID: trackID, Content: "Comment " + strconv.Itoa(index),
		})
		if err != nil {
			t.Fatalf("Create(%d) error = %v", index, err)
		}
	}
	_, err := repository.Create(ctx, comment.Comment{
		ID: uuid.New(), UserID: userID, TrackID: trackID, Content: "One too many",
	})
	if !errors.Is(err, comment.ErrRateLimited) {
		t.Fatalf("Create(over limit) error = %v, want %v", err, comment.ErrRateLimited)
	}
}

func cleanupUsers(t *testing.T, pool *pgxpool.Pool, userIDs ...uuid.UUID) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = pool.Exec(ctx, `
			DELETE FROM comment.report_moderation_actions
			WHERE report_id IN (
				SELECT report.id
				FROM comment.comment_reports AS report
				JOIN comment.comments AS subject ON subject.id = report.comment_id
				WHERE report.reporter_user_id = ANY($1::uuid[])
				   OR subject.user_id = ANY($1::uuid[])
			)
		`, userIDs)
		_, _ = pool.Exec(ctx, `
			DELETE FROM comment.comment_reports
			WHERE reporter_user_id = ANY($1::uuid[])
			   OR comment_id IN (
			       SELECT id FROM comment.comments WHERE user_id = ANY($1::uuid[])
			   )
		`, userIDs)
		_, _ = pool.Exec(ctx, `DELETE FROM comment.comments WHERE user_id = ANY($1::uuid[])`, userIDs)
	})
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
