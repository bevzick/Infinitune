package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/bevzick/Infinitune/services/comment-service/internal/comment"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const maximumReportsPerHour = 10

func (repository *CommentRepository) CreateReport(
	ctx context.Context,
	value comment.Report,
) (comment.Report, error) {
	tx, err := repository.database.Begin(ctx)
	if err != nil {
		return comment.Report{}, fmt.Errorf("begin report comment transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `
		SELECT pg_advisory_xact_lock(hashtextextended('comment-report:' || $1::text, 0))
	`, value.ReporterUserID); err != nil {
		return comment.Report{}, fmt.Errorf("lock comment reporter: %w", err)
	}
	var recentCount int
	if err = tx.QueryRow(ctx, `
		SELECT count(*)
		FROM comment.comment_reports
		WHERE reporter_user_id = $1 AND created_at >= now() - interval '1 hour'
	`, value.ReporterUserID).Scan(&recentCount); err != nil {
		return comment.Report{}, fmt.Errorf("count recent comment reports: %w", err)
	}
	if recentCount >= maximumReportsPerHour {
		return comment.Report{}, comment.ErrReportRateLimited
	}
	var ownerID uuid.UUID
	var status comment.Status
	err = tx.QueryRow(ctx, `
		SELECT user_id, status
		FROM comment.comments
		WHERE id = $1
		FOR SHARE
	`, value.CommentID).Scan(&ownerID, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return comment.Report{}, comment.ErrNotFound
	}
	if err != nil {
		return comment.Report{}, fmt.Errorf("read reported comment: %w", err)
	}
	if status != comment.StatusPublished {
		return comment.Report{}, comment.ErrNotFound
	}
	if ownerID == value.ReporterUserID {
		return comment.Report{}, comment.ErrOwnCommentReport
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO comment.comment_reports (
			id, comment_id, reporter_user_id, reason, description
		) VALUES ($1, $2, $3, $4, $5)
		RETURNING id, comment_id, reporter_user_id, reason, description,
		          status, created_at, updated_at
	`, value.ID, value.CommentID, value.ReporterUserID, value.Reason, value.Description).Scan(
		&value.ID, &value.CommentID, &value.ReporterUserID, &value.Reason,
		&value.Description, &value.Status, &value.CreatedAt, &value.UpdatedAt,
	)
	if err != nil {
		if duplicateReport(err) {
			return comment.Report{}, comment.ErrReportDuplicate
		}
		return comment.Report{}, fmt.Errorf("insert comment report: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return comment.Report{}, fmt.Errorf("commit report comment: %w", err)
	}
	return value, nil
}

func duplicateReport(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) &&
		postgresError.ConstraintName == "comment_reports_comment_reporter_uidx"
}
