package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/bevzick/Infinitune/services/comment-service/internal/comment"
	"github.com/bevzick/Infinitune/services/comment-service/internal/moderation"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (repository *CommentRepository) ListModerationCases(
	ctx context.Context,
	page moderation.PageRequest,
) (moderation.Page, error) {
	var status any
	if page.Status != nil {
		status = string(*page.Status)
	}
	const filter = `($1::comment.report_status IS NULL OR report.status = $1)`
	var total int64
	if err := repository.database.QueryRow(ctx, `
		SELECT count(*) FROM comment.comment_reports AS report WHERE `+filter,
		status).Scan(&total); err != nil {
		return moderation.Page{}, fmt.Errorf("count moderation cases: %w", err)
	}
	rows, err := repository.database.Query(ctx, `
		SELECT report.id, report.comment_id, report.reporter_user_id, report.reason,
		       report.description, report.status, report.created_at, report.updated_at,
		       subject.id, subject.user_id, subject.track_id, subject.parent_id,
		       subject.content, subject.status, subject.created_at, subject.updated_at,
		       subject.deleted_at
		FROM comment.comment_reports AS report
		JOIN comment.comments AS subject ON subject.id = report.comment_id
		WHERE `+filter+`
		ORDER BY report.created_at, report.id
		LIMIT $2 OFFSET $3
	`, status, page.Limit, page.Offset)
	if err != nil {
		return moderation.Page{}, fmt.Errorf("query moderation cases: %w", err)
	}
	defer rows.Close()
	items := make([]moderation.Case, 0, page.Limit)
	for rows.Next() {
		var item moderation.Case
		if err = rows.Scan(
			&item.Report.ID, &item.Report.CommentID, &item.Report.ReporterUserID,
			&item.Report.Reason, &item.Report.Description, &item.Report.Status,
			&item.Report.CreatedAt, &item.Report.UpdatedAt,
			&item.Comment.ID, &item.Comment.UserID, &item.Comment.TrackID,
			&item.Comment.ParentID, &item.Comment.Content, &item.Comment.Status,
			&item.Comment.CreatedAt, &item.Comment.UpdatedAt, &item.Comment.DeletedAt,
		); err != nil {
			return moderation.Page{}, fmt.Errorf("scan moderation case: %w", err)
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return moderation.Page{}, fmt.Errorf("iterate moderation cases: %w", err)
	}
	return moderation.Page{Items: items, Limit: page.Limit, Offset: page.Offset, Total: total}, nil
}

func (repository *CommentRepository) ApplyModerationAction(
	ctx context.Context,
	decision moderation.Decision,
) (moderation.Decision, error) {
	tx, err := repository.database.Begin(ctx)
	if err != nil {
		return moderation.Decision{}, fmt.Errorf("begin comment moderation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var commentID uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT report.comment_id, report.status, subject.status
		FROM comment.comment_reports AS report
		JOIN comment.comments AS subject ON subject.id = report.comment_id
		WHERE report.id = $1
		FOR UPDATE OF report, subject
	`, decision.ReportID).Scan(
		&commentID, &decision.PreviousReportStatus, &decision.PreviousCommentStatus,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return moderation.Decision{}, moderation.ErrNotFound
	}
	if err != nil {
		return moderation.Decision{}, fmt.Errorf("lock comment moderation case: %w", err)
	}
	decision.ResultingReportStatus, decision.ResultingCommentStatus, err = moderationTransition(
		decision.Action, decision.PreviousReportStatus, decision.PreviousCommentStatus,
	)
	if err != nil {
		return moderation.Decision{}, err
	}
	if _, err = tx.Exec(ctx, `
		UPDATE comment.comment_reports
		SET status = $2, updated_at = now()
		WHERE id = $1
	`, decision.ReportID, decision.ResultingReportStatus); err != nil {
		return moderation.Decision{}, fmt.Errorf("update comment report status: %w", err)
	}
	if decision.ResultingCommentStatus != decision.PreviousCommentStatus {
		if _, err = tx.Exec(ctx, `
			UPDATE comment.comments SET status = $2, updated_at = now() WHERE id = $1
		`, commentID, decision.ResultingCommentStatus); err != nil {
			return moderation.Decision{}, fmt.Errorf("update moderated comment status: %w", err)
		}
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO comment.report_moderation_actions (
			id, report_id, moderator_user_id, action, note,
			previous_report_status, resulting_report_status,
			previous_comment_status, resulting_comment_status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at
	`, decision.ID, decision.ReportID, decision.ModeratorUserID, decision.Action,
		decision.Note, decision.PreviousReportStatus, decision.ResultingReportStatus,
		decision.PreviousCommentStatus, decision.ResultingCommentStatus,
	).Scan(&decision.CreatedAt)
	if err != nil {
		return moderation.Decision{}, fmt.Errorf("insert comment moderation audit: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return moderation.Decision{}, fmt.Errorf("commit comment moderation: %w", err)
	}
	return decision, nil
}

func (repository *CommentRepository) ListModerationActions(
	ctx context.Context,
	page moderation.ActionPageRequest,
) (moderation.ActionPage, error) {
	var total int64
	if err := repository.database.QueryRow(ctx,
		`SELECT count(*) FROM comment.report_moderation_actions`).Scan(&total); err != nil {
		return moderation.ActionPage{}, fmt.Errorf("count comment moderation actions: %w", err)
	}
	rows, err := repository.database.Query(ctx, `
		SELECT id, report_id, moderator_user_id, action, note,
		       previous_report_status, resulting_report_status,
		       previous_comment_status, resulting_comment_status, created_at
		FROM comment.report_moderation_actions
		ORDER BY created_at DESC, id DESC
		LIMIT $1 OFFSET $2
	`, page.Limit, page.Offset)
	if err != nil {
		return moderation.ActionPage{}, fmt.Errorf("query comment moderation actions: %w", err)
	}
	defer rows.Close()
	items := make([]moderation.Decision, 0, page.Limit)
	for rows.Next() {
		var item moderation.Decision
		if err = rows.Scan(
			&item.ID, &item.ReportID, &item.ModeratorUserID, &item.Action, &item.Note,
			&item.PreviousReportStatus, &item.ResultingReportStatus,
			&item.PreviousCommentStatus, &item.ResultingCommentStatus, &item.CreatedAt,
		); err != nil {
			return moderation.ActionPage{}, fmt.Errorf("scan comment moderation action: %w", err)
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return moderation.ActionPage{}, fmt.Errorf("iterate comment moderation actions: %w", err)
	}
	return moderation.ActionPage{
		Items: items, Limit: page.Limit, Offset: page.Offset, Total: total,
	}, nil
}

func moderationTransition(
	action moderation.Action,
	reportStatus comment.ReportStatus,
	commentStatus comment.Status,
) (comment.ReportStatus, comment.Status, error) {
	if reportStatus == comment.ReportStatusResolved || reportStatus == comment.ReportStatusRejected {
		return "", "", moderation.ErrConflict
	}
	switch action {
	case moderation.ActionStartReview:
		if reportStatus != comment.ReportStatusPending {
			return "", "", moderation.ErrConflict
		}
		return comment.ReportStatusReviewing, commentStatus, nil
	case moderation.ActionHideComment:
		if commentStatus != comment.StatusPublished {
			return "", "", moderation.ErrConflict
		}
		return comment.ReportStatusReviewing, comment.StatusHidden, nil
	case moderation.ActionRestoreComment:
		if commentStatus != comment.StatusHidden {
			return "", "", moderation.ErrConflict
		}
		return comment.ReportStatusReviewing, comment.StatusPublished, nil
	case moderation.ActionResolveReport:
		return comment.ReportStatusResolved, commentStatus, nil
	case moderation.ActionRejectReport:
		return comment.ReportStatusRejected, commentStatus, nil
	default:
		return "", "", moderation.ErrInvalid
	}
}

var _ moderation.Store = (*CommentRepository)(nil)
